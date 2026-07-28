package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gotd/td/tg"

	"github.com/sorokin-vladimir/tele/internal/domain"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// maxAlbumParts is Telegram's cap on the number of media in one grouped album.
const maxAlbumParts = 10

// albumClass is an attachment's album compatibility class. Telegram refuses to
// mix visual media with documents in one grouped album, so parts of different
// classes must go out as separate albums.
type albumClass int

const (
	classVisual   albumClass = iota // photo, video
	classDocument                   // documents, and anything sent as a file
)

// albumClassOf classifies by the "send as" choice, not the detected kind: a
// photo the user chose to send as a file travels with the documents.
func albumClassOf(a pendingAttachment) albumClass {
	switch a.sendAs {
	case domain.MediaPhoto, domain.MediaVideo:
		return classVisual
	default:
		return classDocument
	}
}

// partitionAlbums splits staged attachments into album-sized groups: stable by
// class (group order follows each class's first occurrence, order within a class
// is preserved), then chunked at maxAlbumParts. Both the mixed-type case and the
// more-than-ten case fall out of the same pass.
func partitionAlbums(atts []pendingAttachment) [][]pendingAttachment {
	if len(atts) == 0 {
		return nil
	}
	var order []albumClass
	buckets := make(map[albumClass][]pendingAttachment, 2)
	for _, a := range atts {
		c := albumClassOf(a)
		if _, seen := buckets[c]; !seen {
			order = append(order, c)
		}
		buckets[c] = append(buckets[c], a)
	}
	var out [][]pendingAttachment
	for _, c := range order {
		bucket := buckets[c]
		for start := 0; start < len(bucket); start += maxAlbumParts {
			end := start + maxAlbumParts
			if end > len(bucket) {
				end = len(bucket)
			}
			out = append(out, bucket[start:end])
		}
	}
	return out
}

// albumPart is one file of an in-flight album: the staged source plus the
// sentinel message that shows its progress.
type albumPart struct {
	att        pendingAttachment
	sentinelID int
}

// albumSend is the state of one in-flight multi-file send. Only one runs at a
// time: parts upload sequentially, which keeps the aggregate percentage honest
// and leaves cancellation meaningful.
type albumSend struct {
	serial       int
	peer         domain.Peer
	chatID       int64
	caption      string
	entities     []domain.MessageEntity
	replyToMsgID int
	groups       [][]albumPart
	group        int // index of the group currently uploading
	part         int // index within that group
	uploaded     []internaltg.AlbumItem
	survivors    []albumPart
	totalBytes   int64
	doneBytes    int64
	statusSerial int
	okCount      int
	failCount    int
	cancel       context.CancelFunc
}

// albumPartUploadedMsg reports the outcome of one part's UploadFile+UploadMedia.
type albumPartUploadedMsg struct {
	serial int
	media  tg.InputMediaClass
	err    error
}

// albumGroupSentMsg reports the outcome of one group's send.
type albumGroupSentMsg struct {
	serial int
	ids    []int
	err    error
}

// albumRefreshedMsg carries the server's view of a just-sent group, for the
// media refs and the grouped_id that collapses the parts into one bubble. ids
// are the messages the refresh was asked for, so a failed refresh can still
// clear their upload placeholders.
type albumRefreshedMsg struct {
	serial int
	ids    []int
	msgs   []domain.Message
}

// StatusBarTransferActive reports whether the status bar shows a transfer
// indicator (test accessor).
func (m RootModel) StatusBarTransferActive() bool { return m.statusBar.DownloadActive() }

// startAlbumSend stages one optimistic bubble per file, then starts the
// sequential upload pipeline. The caller clears the staged queue.
func (m RootModel) startAlbumSend(peer domain.Peer, caption string, entities []domain.MessageEntity, replyToMsgID int) (RootModel, tea.Cmd) {
	if m.tgClient == nil || m.st == nil || len(m.pendingAttachments) == 0 {
		return m, nil
	}
	chatID := m.currentChatID
	groups := partitionAlbums(m.pendingAttachments)

	m.nextAlbumSer++
	as := &albumSend{
		serial:       m.nextAlbumSer,
		peer:         peer,
		chatID:       chatID,
		caption:      caption,
		entities:     entities,
		replyToMsgID: replyToMsgID,
	}
	first := true
	for _, g := range groups {
		parts := make([]albumPart, 0, len(g))
		for _, a := range g {
			m.nextSentinel--
			sentinelID := m.nextSentinel
			// Telegram renders an album's caption from its first part, so the
			// optimistic bubbles carry it the same way: on the first one only.
			// Without this the caption vanishes until the chat is reloaded.
			var (
				partText     string
				partEntities []domain.MessageEntity
				partReplyTo  int
			)
			if first {
				partText, partEntities, partReplyTo = caption, entities, replyToMsgID
				first = false
			}
			m.st.AppendMessage(domain.Message{
				ID:           sentinelID,
				ChatID:       chatID,
				Text:         partText,
				Entities:     partEntities,
				ReplyToMsgID: partReplyTo,
				Date:         time.Now(),
				IsOut:        true,
				LocalMedia: &domain.LocalMedia{
					Path:        a.path,
					Kind:        a.sendAs,
					FileName:    a.name,
					Size:        a.size,
					UploadState: domain.UploadUploading,
				},
			})
			as.totalBytes += a.size
			parts = append(parts, albumPart{att: a, sentinelID: sentinelID})
		}
		as.groups = append(as.groups, parts)
	}
	m.chat.SetMessages(m.st.Messages(chatID))

	uploadCtx, cancel := context.WithCancel(m.ctx)
	as.cancel = cancel
	as.statusSerial = m.statusBar.StartTransfer(as.progressLabel(0))
	m.album = as
	m.albumCtx = uploadCtx
	return m, m.uploadNextAlbumPartCmd()
}

// progressLabel renders the status-bar label: which file of how many, and the
// percentage over the whole batch's bytes including what the current part has
// sent so far, so the number advances within a file rather than in steps.
func (a *albumSend) progressLabel(curSent int64) string {
	pct := 0
	if a.totalBytes > 0 {
		pct = int(float64(a.doneBytes+curSent) / float64(a.totalBytes) * 100)
	}
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("up %d/%d %d%%", a.uploadIndex()+1, a.partCount(), pct)
}

func (a *albumSend) partCount() int {
	n := 0
	for _, g := range a.groups {
		n += len(g)
	}
	return n
}

// uploadIndex is the flat index of the part currently uploading.
func (a *albumSend) uploadIndex() int {
	n := 0
	for i := 0; i < a.group && i < len(a.groups); i++ {
		n += len(a.groups[i])
	}
	return n + a.part
}

func (a *albumSend) currentPart() (albumPart, bool) {
	if a.group >= len(a.groups) || a.part >= len(a.groups[a.group]) {
		return albumPart{}, false
	}
	return a.groups[a.group][a.part], true
}

// ownsSentinel reports whether sentinelID belongs to this album send.
func (a *albumSend) ownsSentinel(sentinelID int) bool {
	for _, g := range a.groups {
		for _, p := range g {
			if p.sentinelID == sentinelID {
				return true
			}
		}
	}
	return false
}

// uploadNextAlbumPartCmd uploads the current part and converts it into a
// server-side media ref. Progress reuses the sentinel-keyed channel pump the
// single-file path already runs.
func (m RootModel) uploadNextAlbumPartCmd() tea.Cmd {
	as := m.album
	part, ok := as.currentPart()
	if !ok {
		return nil
	}
	client := m.tgClient
	ctx := m.albumCtx
	peer := as.peer
	serial := as.serial
	att := part.att
	sentinelID := part.sentinelID

	progressCh := make(chan uploadProgressMsg, 8)
	m.uploadProgress[sentinelID] = progressCh

	upload := func() tea.Msg {
		f, err := client.UploadFile(ctx, internaltg.UploadParams{
			Path: att.path,
			OnProgress: func(sent, total int64) {
				select {
				case progressCh <- uploadProgressMsg{sentinelID: sentinelID, sent: sent, total: total}:
				default:
				}
			},
		})
		close(progressCh)
		if err != nil {
			return albumPartUploadedMsg{serial: serial, err: err}
		}
		var uploaded tg.InputMediaClass
		if att.sendAs == domain.MediaVideo {
			uploaded, err = videoBuildMediaCtx(att.path, att.name, att.mime)(ctx, client, f)
			if err != nil {
				return albumPartUploadedMsg{serial: serial, err: err}
			}
		} else {
			build, ok := mediaBuilderFor(&att)
			if !ok {
				return albumPartUploadedMsg{serial: serial, err: internaltg.ErrUnsupportedAlbumMedia}
			}
			uploaded = build(f)
		}
		// sendMultiMedia rejects inputMediaUploaded*, so every part takes this hop
		// even when the group turns out to hold a single survivor.
		ref, err := client.UploadMedia(ctx, peer, uploaded)
		if err != nil {
			return albumPartUploadedMsg{serial: serial, err: err}
		}
		return albumPartUploadedMsg{serial: serial, media: ref}
	}
	return tea.Batch(upload, recvProgressCmd(progressCh))
}

func (m RootModel) handleAlbumPartUploaded(msg albumPartUploadedMsg) (RootModel, tea.Cmd) {
	as := m.album
	if as == nil || as.serial != msg.serial {
		return m, nil // a cancelled or superseded send
	}
	part, ok := as.currentPart()
	if !ok {
		return m, nil
	}
	delete(m.uploadProgress, part.sentinelID)
	as.doneBytes += part.att.size
	if msg.err != nil {
		as.failCount++
		m.st.MarkLocalMediaFailed(part.sentinelID)
		if as.chatID == m.currentChatID {
			m.chat.SetMessagesKeepScroll(m.st.Messages(as.chatID))
		}
	} else {
		item := internaltg.AlbumItem{Media: msg.media}
		// Telegram shows the album caption from its first part, and the caption
		// belongs to the send as a whole: it rides on the first survivor overall.
		if as.group == 0 && len(as.uploaded) == 0 {
			item.Caption = as.caption
			item.Entities = as.entities
		}
		as.uploaded = append(as.uploaded, item)
		as.survivors = append(as.survivors, part)
	}
	as.part++
	if _, more := as.currentPart(); more {
		m.statusBar.UpdateTransfer(as.statusSerial, as.progressLabel(0))
		return m, m.uploadNextAlbumPartCmd()
	}
	return m, m.sendAlbumGroupCmd()
}

// sendAlbumGroupCmd sends the survivors of the current group: a lone survivor
// goes out as a plain media message, since a one-part album is just a message.
func (m RootModel) sendAlbumGroupCmd() tea.Cmd {
	as := m.album
	serial := as.serial
	if len(as.uploaded) == 0 {
		return func() tea.Msg { return albumGroupSentMsg{serial: serial} }
	}
	client := m.tgClient
	ctx := m.albumCtx
	peer := as.peer
	items := as.uploaded
	replyTo := 0
	if as.group == 0 {
		replyTo = as.replyToMsgID
	}
	if len(items) == 1 {
		it := items[0]
		return func() tea.Msg {
			id, err := client.SendMedia(ctx, internaltg.SendMediaParams{
				Peer: peer, Media: it.Media, Caption: it.Caption,
				ReplyToMsgID: replyTo, Entities: it.Entities,
			})
			if err != nil {
				return albumGroupSentMsg{serial: serial, err: err}
			}
			return albumGroupSentMsg{serial: serial, ids: []int{id}}
		}
	}
	return func() tea.Msg {
		ids, err := client.SendAlbum(ctx, internaltg.SendAlbumParams{
			Peer: peer, Items: items, ReplyToMsgID: replyTo,
		})
		if err != nil {
			return albumGroupSentMsg{serial: serial, err: err}
		}
		return albumGroupSentMsg{serial: serial, ids: ids}
	}
}

func (m RootModel) handleAlbumGroupSent(msg albumGroupSentMsg) (RootModel, tea.Cmd) {
	as := m.album
	if as == nil || as.serial != msg.serial {
		return m, nil
	}
	if msg.err != nil {
		for _, p := range as.survivors {
			as.failCount++
			m.st.MarkLocalMediaFailed(p.sentinelID)
		}
		if as.chatID == m.currentChatID {
			m.chat.SetMessagesKeepScroll(m.st.Messages(as.chatID))
		}
		return m.advanceAlbumGroup(nil)
	}
	ids := msg.ids
	for i, p := range as.survivors {
		if i >= len(ids) || ids[i] == 0 {
			// The server confirmed the send but not this ID; drop the placeholder
			// rather than leaving a bubble stuck at 100%.
			m.st.ClearLocalMedia(p.sentinelID)
			continue
		}
		as.okCount++
		m.st.UpdateMessageID(as.chatID, p.sentinelID, ids[i])
		// Keep the local bubble (now at 100%) until the server media arrives, so
		// the photo never blanks out to a caption-only message in between.
		m.st.UpdateLocalMediaProgress(ids[i], 1)
	}
	if as.chatID == m.currentChatID {
		m.chat.SetMessages(m.st.Messages(as.chatID))
	}
	sent := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != 0 {
			sent = append(sent, id)
		}
	}
	return m.advanceAlbumGroup(sent)
}

// advanceAlbumGroup refreshes the group just sent (if any) and moves on to the
// next group, or finishes the send.
func (m RootModel) advanceAlbumGroup(sentIDs []int) (RootModel, tea.Cmd) {
	as := m.album
	var refreshCmd tea.Cmd
	if len(sentIDs) > 0 {
		// The refresh runs on the app context, not the album's: the album context
		// is cancelled the moment the send finishes (and by an explicit cancel),
		// while this fetch is post-send bookkeeping for messages that already
		// exist server-side. Tying it to the album context made it fail every
		// time, leaving the parts with no media, no grouping and no caption.
		client, ctx, peer, serial := m.tgClient, m.ctx, as.peer, as.serial
		ids := sentIDs
		refreshCmd = func() tea.Msg {
			msgs, err := client.RefreshMessages(ctx, peer, ids)
			if err != nil {
				return albumRefreshedMsg{serial: serial, ids: ids}
			}
			return albumRefreshedMsg{serial: serial, ids: ids, msgs: msgs}
		}
	}
	as.group++
	as.part = 0
	as.uploaded = nil
	as.survivors = nil
	if as.group < len(as.groups) {
		m.statusBar.UpdateTransfer(as.statusSerial, as.progressLabel(0))
		return m, tea.Batch(refreshCmd, m.uploadNextAlbumPartCmd())
	}
	nm, finishCmd := m.finishAlbumSend()
	return nm, tea.Batch(refreshCmd, finishCmd)
}

func (m RootModel) handleAlbumRefreshed(msg albumRefreshedMsg) (RootModel, tea.Cmd) {
	if m.st == nil {
		return m, nil
	}
	chatID := m.currentChatID
	if m.album != nil && m.album.serial == msg.serial {
		chatID = m.album.chatID
	}
	if len(msg.msgs) == 0 {
		// The refresh failed: drop the placeholders so the bubbles at least stop
		// showing a stuck progress bar, exactly as the single-file path does.
		for _, id := range msg.ids {
			m.st.ClearLocalMedia(id)
		}
		if chatID == m.currentChatID {
			m.chat.SetMessagesKeepScroll(m.st.Messages(chatID))
		}
		return m, nil
	}
	for _, r := range msg.msgs {
		m.st.AdoptServerMedia(chatID, r.ID, r.Photo, r.Document, r.Media)
		if r.GroupedID != 0 {
			m.st.SetGroupedID(chatID, r.ID, r.GroupedID)
		}
	}
	if chatID != m.currentChatID {
		return m, nil
	}
	m.chat.SetMessagesKeepScroll(m.st.Messages(chatID))
	// Pull the images so the album renders inline instead of as empty tiles.
	want := make(map[int]struct{}, len(msg.msgs))
	for _, r := range msg.msgs {
		want[r.ID] = struct{}{}
	}
	var fresh []domain.Message
	for _, mm := range m.st.Messages(chatID) {
		if _, ok := want[mm.ID]; ok {
			fresh = append(fresh, mm)
		}
	}
	return m, m.pendingDownloadCmds(fresh)
}

// finishAlbumSend releases the status-bar slot and reports the outcome: silence
// on a clean send, a warning on a partial one, an error when nothing landed.
func (m RootModel) finishAlbumSend() (RootModel, tea.Cmd) {
	as := m.album
	if as == nil {
		return m, nil
	}
	m.statusBar.ClearTransfer(as.statusSerial)
	ok, failed := as.okCount, as.failCount
	if as.cancel != nil {
		as.cancel()
	}
	m.album = nil
	m.albumCtx = nil
	if failed == 0 {
		return m, nil
	}
	if ok == 0 {
		return m, func() tea.Msg {
			return StatusErrMsg{Text: "album send failed", Sev: components.SeverityError}
		}
	}
	total := ok + failed
	return m, func() tea.Msg {
		return StatusErrMsg{
			Text: fmt.Sprintf("%d of %d sent", ok, total),
			Sev:  components.SeverityWarning,
		}
	}
}
