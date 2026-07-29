package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// handleDelta renders one projection delta. It performs no mutation and reads no
// domain state: everything it needs is on the delta, addressed to a subscription
// the client asked for.
func (m RootModel) handleDelta(d project.Delta) (RootModel, tea.Cmd) {
	switch {
	case d.ChatList != nil && d.Sub == m.chatListSub:
		return m.handleChatListDelta(d.ChatList)
	case d.Chat != nil && d.Sub == m.chatSub:
		return m.handleChatDelta(d.Chat)
	}
	// A delta for a subscription this model no longer holds: the window was
	// replaced (a chat switch) and the reply raced the unsubscribe. Dropping it
	// is correct — the new subscription's Reset carries the truth.
	return m, nil
}

func (m RootModel) handleChatListDelta(d *project.ChatListDelta) (RootModel, tea.Cmd) {
	switch d.Kind {
	case project.ChatListReset:
		m.chatList.SetWindow(d.Offset, d.Total, d.Rows)
		m.chatList.SetActive(m.currentChatID)

	case project.ChatListRow:
		m.chatList.SetRow(d.Row)

	case project.ChatListFolders:
		if m.folderBar == nil {
			break
		}
		m.folderBar.SetUnreadCounts(d.Folders.Unread)
		m.folderBar.SetArchivePresent(d.Folders.ArchivePresent)
		// The Archive folder was showing and just became empty (the last archived
		// chat was unarchived): fall back to All Chats so the user is not
		// stranded on an empty list.
		if m.activeFolder == domainArchiveFolderID && !d.Folders.ArchivePresent {
			return m.selectFolder(0)
		}
	}
	return m, nil
}

// handleIncoming reacts to a message that arrived in a chat the user is not
// looking at: the row flashes so the eye can follow the reorder (#39), and an
// unmuted one also pops a toast (#59). Whether it deserves a toast is the
// owner's decision, not this model's.
func (m RootModel) handleIncoming(in core.Incoming) (RootModel, tea.Cmd) {
	m.chatList.HighlightChat(in.ChatID)
	m.chatHighlightSerial++
	cmds := []tea.Cmd{chatHighlightFadeCmd(m.chatHighlightSerial)}
	if in.Notify {
		cmds = append(cmds, m.showInAppNotify(in))
	}
	return m, tea.Batch(cmds...)
}

// handleFailure renders work the owner could not finish. The owner reports what
// failed in domain terms (#191); saying it is the client's job.
func (m RootModel) handleFailure(f core.Failure) (RootModel, tea.Cmd) {
	text, _, ok := errText(f.Op, f.Err)
	if !ok {
		// A cancelled operation is not a failure and must not blank out the
		// pane with an empty error banner.
		return m, nil
	}
	return m.handleChatLoadErr(chatLoadErrMsg{chatID: f.ChatID, text: text})
}

// notifyOpenMsg is emitted when a notify toast is clicked: it dismisses the
// toast and opens the target chat (#59 click-to-open).
type notifyOpenMsg struct {
	chatID int64
	title  string
	serial int
}

// showInAppNotify adds a top-right notify toast for an incoming message in an
// inactive chat and returns its auto-dismiss command. The whole toast is a click
// target that opens the chat.
func (m RootModel) showInAppNotify(in core.Incoming) tea.Cmd {
	title := in.Title
	if title == "" {
		title = "New message"
	}
	serial := m.toasts.Add(components.ToastNotify, title+"\n"+in.Preview)
	m.toasts.SetClick(serial, notifyOpenMsg{chatID: in.ChatID, title: in.Title, serial: serial})
	return tea.Tick(durationFor(components.SeverityInfo), func(time.Time) tea.Msg {
		return ClearStatusErrMsg{Serial: serial}
	})
}
