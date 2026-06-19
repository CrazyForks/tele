package ui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	vmedia "github.com/sorokin-vladimir/tele/internal/media"
	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/media"
)

// useInAppVideoPlayer reports whether a video should open in the in-app modal
// (Kitty graphics + ffmpeg available) rather than the external system player.
func useInAppVideoPlayer(mode media.Mode, hasFFmpeg bool) bool {
	return mode == media.ModeKitty && hasFFmpeg
}

// videoPlayer is the in-app video modal overlay state. Phase 3a holds a single
// (first) frame; Phase 3b adds the streaming source, play state, and position.
type videoPlayer struct {
	docID   int64
	path    string // downloaded temp file, for the external-player fallback (o)
	durSecs int
	title   string // sender name, shown on the top border
	frame   image.Image
	cols    int
	rows    int

	source     *vmedia.FrameSource
	playing    bool
	posFrames  int // frames shown since the loop's start (position = posFrames/videoFPS)
	gen        int // bumped on (re)open/close to drop stale ticks
	spinnerIdx int // loading-spinner glyph index while no frame has been shown
}

// videoFPS is the modal playback rate. videoFrameInterval is the tick period.
const videoFPS = 15

var videoFrameInterval = time.Second / videoFPS

func videoTickCmd(gen int) tea.Cmd {
	return tea.Tick(videoFrameInterval, func(time.Time) tea.Msg { return videoTickMsg{gen: gen} })
}

// fmtClock renders whole seconds as m:ss.
func fmtClock(secs int) string {
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// videoModalBox sizes the modal image to fit ~80% of the terminal while keeping
// the source aspect ratio, reusing the shared PhotoBox sizing.
func (m RootModel) videoModalBox(imgW, imgH int) (int, int) {
	cw, ch := media.CellPx()
	maxCols := m.width * 4 / 5
	maxRows := m.height * 4 / 5
	return media.PhotoBox(imgW, imgH, maxCols, maxRows, 1600, cw, ch, media.CellAspect())
}

// downloadVideoFileCmd streams a video's bytes to a temp file for the in-app
// player. Mirrors downloadGifFileCmd but carries the duration for the progress
// bar and yields a videoFileReadyMsg.
func downloadVideoFileCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef, tmpDir string, durSecs int, sender string) tea.Cmd {
	return func() tea.Msg {
		f, err := createTempMediaFile(tmpDir, ".mp4")
		if err != nil {
			return nil
		}
		name := f.Name()
		_, _, derr := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.DocumentRef) (struct{}, error) {
				if _, serr := f.Seek(0, 0); serr != nil {
					return struct{}{}, serr
				}
				if terr := f.Truncate(0); terr != nil {
					return struct{}{}, terr
				}
				return struct{}{}, client.DownloadDocumentToFile(ctx, r, f)
			},
			pickDocumentRef,
		)
		_ = f.Close()
		if derr != nil {
			return nil
		}
		return videoFileReadyMsg{docID: ref.ID, msgID: msgID, path: name, durSecs: durSecs, sender: sender}
	}
}

// probeVideoCmd reads the video's real dimensions so the modal box matches its
// aspect. On failure it falls back to a 16:9 box.
func probeVideoCmd(ctx context.Context, docID int64, path string) tea.Cmd {
	return func() tea.Msg {
		meta, err := vmedia.ProbeVideo(ctx, path)
		w, h := 1920, 1080
		if err == nil && meta.Width > 0 && meta.Height > 0 {
			w, h = meta.Width, meta.Height
		}
		return videoProbedMsg{docID: docID, path: path, w: w, h: h}
	}
}

// videoPlayerKey is the stable KittyStore key for the modal's image id, distinct
// from any message photo/document id.
const videoPlayerKey int64 = -1000

func mustCellW() float64 { w, _ := media.CellPx(); return w }
func mustCellH() float64 { _, h := media.CellPx(); return h }

// transmitFrameToID writes a Kitty transmit-and-place for a specific id via
// tea.Raw (approach A: overwriting the same id updates the placement in place).
func transmitFrameToID(id uint32, frame image.Image, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		seq, err := media.TransmitSeq(id, frame, cols, rows)
		if err != nil {
			return nil
		}
		return tea.Raw(seq)()
	}
}

// selectedVideoInfo returns the selected message's video duration (seconds) and
// sender display name, or zero values if unknown.
func (m RootModel) selectedVideoInfo() (int, string) {
	if m.st == nil || m.chat == nil {
		return 0, ""
	}
	id := m.chat.SelectedMessageID()
	for _, msg := range m.st.Messages(m.currentChatID) {
		if msg.ID == id {
			dur := 0
			if msg.Media != nil {
				dur = msg.Media.Duration
			}
			return dur, msg.SenderName
		}
	}
	return 0, ""
}

// openVideoPlayerCmd downloads the selected video and kicks off first-frame decode.
func (m RootModel) openVideoPlayerCmd(ref store.DocumentRef, msgID, durSecs int, sender string) tea.Cmd {
	return downloadVideoFileCmd(m.ctx, m.tgClient, m.currentPeer(), msgID, ref, m.tmpDir, durSecs, sender)
}

func (m RootModel) handleVideoFileReady(msg videoFileReadyMsg) (RootModel, tea.Cmd) {
	// Open the modal shell now (spinner shows until the first frame), then probe
	// real dimensions so the box (and therefore the decode size) keeps the aspect.
	m.videoPlayer = &videoPlayer{docID: msg.docID, path: msg.path, durSecs: msg.durSecs, title: msg.sender}
	return m, probeVideoCmd(m.ctx, msg.docID, msg.path)
}

// handleVideoProbed sizes the box to the real aspect and opens the frame stream.
func (m RootModel) handleVideoProbed(msg videoProbedMsg) (RootModel, tea.Cmd) {
	if m.videoPlayer == nil || m.videoPlayer.docID != msg.docID {
		return m, nil
	}
	cols, rows := m.videoModalBox(msg.w, msg.h)
	m.videoPlayer.cols = cols
	m.videoPlayer.rows = rows
	w := int(float64(cols) * mustCellW())
	h := int(float64(rows) * mustCellH())
	src, err := vmedia.OpenFrameSource(m.ctx, msg.path, w, h, videoFPS)
	if err != nil {
		return m, nil // leave the spinner; q closes
	}
	m.videoPlayer.source = src
	m.videoPlayer.playing = true
	m.videoPlayer.posFrames = 0
	m.videoPlayer.gen++
	return m, videoTickCmd(m.videoPlayer.gen)
}

// handleVideoTick pulls and shows the next frame, then re-arms. At EOF it loops
// by reopening the source. Pausing stops re-arming (ffmpeg backpressures).
func (m RootModel) handleVideoTick(msg videoTickMsg) (RootModel, tea.Cmd) {
	vp := m.videoPlayer
	if vp == nil || vp.source == nil || msg.gen != vp.gen || !vp.playing {
		return m, nil
	}
	frame, ok := vp.source.Next()
	if !ok {
		// End of stream: loop from the start.
		_ = vp.source.Close()
		w := int(float64(vp.cols) * mustCellW())
		h := int(float64(vp.rows) * mustCellH())
		src, err := vmedia.OpenFrameSource(m.ctx, vp.path, w, h, videoFPS)
		if err != nil {
			return m, nil
		}
		vp.source = src
		vp.posFrames = 0
		return m, videoTickCmd(vp.gen)
	}
	vp.frame = frame
	vp.posFrames++
	m.imageCache[videoPlayerKey] = frame
	id := m.kittyStore.IDFor(videoPlayerKey)
	return m, tea.Batch(transmitFrameToID(id, frame, vp.cols, vp.rows), videoTickCmd(vp.gen))
}

// videoSpinnerGlyph returns the loading-spinner glyph for the given index,
// reusing the GIF spinner frames.
func videoSpinnerGlyph(idx int) string {
	return gifSpinnerFrames[idx%len(gifSpinnerFrames)]
}

// updateVideoSpinner advances the modal's loading spinner while no frame has been
// shown yet. Driven off the existing SpinnerTickMsg cadence — no extra ticker.
func (m *RootModel) updateVideoSpinner() {
	if m.videoPlayer != nil && m.videoPlayer.frame == nil {
		m.videoPlayer.spinnerIdx++
	}
}

// togglePlay flips play/pause.
func (m RootModel) togglePlay() RootModel {
	if m.videoPlayer != nil {
		m.videoPlayer.playing = !m.videoPlayer.playing
	}
	return m
}

// closeVideoPlayer tears down the overlay, stops ffmpeg, and drops the frame.
func (m RootModel) closeVideoPlayer() RootModel {
	if m.videoPlayer != nil {
		if m.videoPlayer.source != nil {
			_ = m.videoPlayer.source.Close()
		}
		m.videoPlayer.gen++
		delete(m.imageCache, videoPlayerKey)
		m.videoPlayer = nil
	}
	return m
}

// handleVideoPlayerKey handles keys while the modal is open: q/esc close, o opens
// the external player, space toggles play/pause.
func (m RootModel) handleVideoPlayerKey(keyStr string) (RootModel, tea.Cmd) {
	switch keyStr {
	case "q", "esc":
		return m.closeVideoPlayer(), nil
	case "o":
		if m.videoPlayer != nil && m.videoPlayer.path != "" {
			openPath(m.videoPlayer.path)
		}
		return m, nil
	case " ", "space":
		m = m.togglePlay()
		if m.videoPlayer != nil && m.videoPlayer.playing {
			return m, videoTickCmd(m.videoPlayer.gen)
		}
		return m, nil
	}
	return m, nil
}

// videoFooterHints renders the modal hint bar in the app's status-bar style; the
// space action reflects the current state (pause while playing, play while paused).
func videoFooterHints(playing bool) string {
	space := "play"
	if playing {
		space = "pause"
	}
	return components.HintBar([][2]string{{"space", space}, {"o", "external"}, {"q", "close"}})
}

// videoProgressRow renders a full-width filled bar for posFrames/totalFrames.
func videoProgressRow(posFrames, totalFrames, width int) string {
	if width < 1 {
		width = 1
	}
	frac := 0.0
	if totalFrames > 0 {
		frac = float64(posFrames) / float64(totalFrames)
		if frac > 1 {
			frac = 1
		}
	}
	filled := int(frac*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat("▰", filled) + strings.Repeat("▱", width-filled)
}

// modalBorder builds one border edge of width innerW+2: a corner, an optional
// left label, a mid-char fill, an optional right label, and the closing corner.
// Labels are dropped (right first, then left) when they would exceed innerW.
func modalBorder(cornerL, mid, cornerR, leftLabel, rightLabel string, innerW int) string {
	ll := lipgloss.Width(leftLabel)
	rl := lipgloss.Width(rightLabel)
	if ll+rl > innerW {
		rightLabel, rl = "", 0
	}
	if ll+rl > innerW {
		leftLabel, ll = "", 0
	}
	fill := innerW - ll - rl
	if fill < 0 {
		fill = 0
	}
	return cornerL + leftLabel + strings.Repeat(mid, fill) + rightLabel + cornerR
}

// videoModalBoxLines renders the bordered modal: top border with the sender on it,
// `rows` content rows (each = left border + the cols-wide placeholder grid row +
// right border), and a bottom border with hints on the left and time on the right.
// Each line is innerW+2 display cells wide.
func videoModalBoxLines(content []string, innerW int, title, hints, timeStr string) []string {
	bd := lipgloss.RoundedBorder()
	lines := make([]string, 0, len(content)+2)
	lines = append(lines, modalBorder(bd.TopLeft, bd.Top, bd.TopRight, label(title), "", innerW))
	for _, c := range content {
		lines = append(lines, bd.Left+c+bd.Right)
	}
	lines = append(lines, modalBorder(bd.BottomLeft, bd.Bottom, bd.BottomRight, label(hints), label(timeStr), innerW))
	return lines
}

// label wraps a non-empty border label in single spaces; empty stays empty.
func label(s string) string {
	if s == "" {
		return ""
	}
	return " " + s + " "
}

// videoPlayerView composites the bordered modal over base (the chat), centered.
// Geometry uses the known cols/rows + integer stamping so Kitty placeholders are
// never measured with lipgloss.
func (m RootModel) videoPlayerView(base string) string {
	vp := m.videoPlayer
	if vp == nil {
		return base
	}
	// Image rows, or a spinner while still loading.
	var content []string
	if vp.frame != nil {
		id := m.kittyStore.IDFor(videoPlayerKey)
		content = media.PlaceholderLines(id, vp.cols, vp.rows)
	} else {
		blank := strings.Repeat(" ", vp.cols)
		content = make([]string, vp.rows)
		for i := range content {
			content[i] = blank
		}
		if vp.rows > 0 {
			line := videoSpinnerGlyph(vp.spinnerIdx) + " loading…"
			if pad := vp.cols - lipgloss.Width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			content[0] = line
		}
	}
	// Progress bar row under the image (inside the box).
	content = append(content, videoProgressRow(vp.posFrames, vp.durSecs*videoFPS, vp.cols))

	posSecs := vp.posFrames / videoFPS
	timeStr := fmtClock(posSecs) + " / " + fmtClock(vp.durSecs)
	box := videoModalBoxLines(content, vp.cols, vp.title, videoFooterHints(vp.playing), timeStr)

	boxW := vp.cols + 2
	left := (m.width - boxW) / 2
	if left < 0 {
		left = 0
	}
	top := (m.height - len(box)) / 2
	if top < 0 {
		top = 0
	}
	return stampBoxOverlay(base, box, top, left, boxW, m.height)
}
