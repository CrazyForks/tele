package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// subscribeChat opens the chat:<id> subscription for a newly opened chat,
// anchored on the first unread so the pane lands on the separator. The previous
// subscription is dropped: a window nobody looks at must stop costing.
//
// fallbackPeer addresses a chat the owner does not hold — a contact found by
// search — and is zero for every chat opened from the list.
func (m *RootModel) subscribeChat(chatID int64, fallbackPeer domain.Peer) {
	// TRANSITIONAL (#198): the composer's outgoing requests still address a
	// peer, which no projection carries. Resolved from the store, or from the
	// message when the store has never heard of this chat.
	peer := fallbackPeer
	if m.st != nil {
		if c, ok := m.st.GetChat(chatID); ok {
			peer = c.Peer
		}
	}
	m.chat.SetPeer(peer)

	if m.owner == nil {
		return
	}
	if m.chatSub != 0 {
		m.owner.Unsubscribe(m.chatSub)
		m.chatSub = 0
	}
	m.chatWindow = project.ChatWindow{
		ChatID: chatID,
		Anchor: project.Anchor{Kind: project.AnchorFirstUnread},
		Before: m.historyLimit,
		After:  0,
	}
	m.chatSub = m.owner.Subscribe(m.chatWindow)
}

// widenChatWindow asks for another page above the current window. It replaces
// the whole window rather than describing an increment, so repeating it is
// harmless and equivalent to resubscribing.
func (m *RootModel) widenChatWindow() {
	if m.owner == nil || m.chatSub == 0 {
		return
	}
	m.chatWindow.Before += m.historyLimit
	m.owner.MoveWindow(m.chatSub, m.chatWindow)
}

// handleChatDelta renders one chat:<id> delta into the chat pane.
func (m RootModel) handleChatDelta(d *project.ChatDelta) (RootModel, tea.Cmd) {
	switch d.Kind {
	case project.ChatReset:
		c := d.Contents
		m.chat.SetHeader(screens.ChatHeader{
			ChatID:          c.ChatID,
			Title:           c.Title,
			IsUser:          c.IsUser,
			IsGroup:         c.IsGroup,
			Online:          c.Online,
			ReadOutboxMaxID: c.ReadOutboxMaxID,
		})
		m.chat.SeedDraft(c.ChatID, c.Draft)
		m.chat.SetInboxReadMaxID(c.ReadInboxMaxID)
		m.chatMsgs = c.Messages
		m.chat.SetMessages(m.chatMsgs)
		m.chat.SetLoading(false)
		m.chat.SetLoadError("")
		m.applyTypingLabel(c.Typing)
		// The window was anchored on the first unread: put that message on
		// screen rather than the tail.
		if c.AnchorMsgID != 0 && c.AnchorMsgID <= c.ReadInboxMaxID {
			break
		}
		if c.ReadInboxMaxID > 0 {
			m.chat.ScrollToFirstUnread(c.ReadInboxMaxID)
		}
		// A chat may open with a GIF already selected (newest message) and its
		// thumbnail already cached from a prior visit; start its animation here
		// since no key event will.
		nm, gifCmd := m.ensureGifAnimForSelection()
		return nm, tea.Batch(nm.markReadCmd(), nm.pendingDownloadCmds(c.Messages), gifCmd)

	case project.ChatOlder:
		if len(d.Messages) == 0 {
			break
		}
		m.chatMsgs = append(append([]domain.Message{}, d.Messages...), m.chatMsgs...)
		m.chat.PrependMessages(d.Messages) // dedups + preserves viewport position
		return m, m.pendingDownloadCmds(d.Messages)

	case project.ChatNewer:
		if len(d.Messages) == 0 {
			break
		}
		m.chatMsgs = append(m.chatMsgs, d.Messages...)
		m.chat.SetMessagesKeepScroll(m.chatMsgs)
		return m, m.pendingDownloadCmds(d.Messages)

	case project.ChatAppend:
		m.chatMsgs = append(m.chatMsgs, d.Message)
		// SetMessages, not KeepScroll: a message arriving in the chat you are
		// reading should bring the view to it.
		m.chat.SetMessages(m.chatMsgs)
		cmds := []tea.Cmd{m.markReadCmd(), m.pendingDownloadCmds([]domain.Message{d.Message})}
		if m.focus == FocusChat && d.Message.Mentioned {
			cmds = append(cmds, m.readMentionsCmd(m.currentChatID))
		}
		return m, tea.Batch(cmds...)

	case project.ChatUpdate:
		// An edit or a reaction on a message already on screen: re-render in
		// place and keep the scroll position. An arriving reaction also needs
		// marking read while the user is looking at the chat (#199).
		for i := range m.chatMsgs {
			if m.chatMsgs[i].ID == d.Message.ID {
				m.chatMsgs[i] = d.Message
				break
			}
		}
		m.chat.SetMessagesKeepScroll(m.chatMsgs)
		if m.focus == FocusChat && d.Message.HasUnreadReactions {
			return m, m.readReactionsCmd(m.currentChatID)
		}

	case project.ChatRemove:
		gone := make(map[int]struct{}, len(d.MsgIDs))
		for _, id := range d.MsgIDs {
			gone[id] = struct{}{}
			m.chat.RemoveMessage(id)
		}
		kept := m.chatMsgs[:0]
		for _, msg := range m.chatMsgs {
			if _, drop := gone[msg.ID]; !drop {
				kept = append(kept, msg)
			}
		}
		m.chatMsgs = kept

	case project.ChatRead:
		m.chat.SetInboxReadMaxID(d.ReadInboxMaxID)
		m.chat.SetOutboxReadMaxID(d.ReadOutboxMaxID)

	case project.ChatDraft:
		// Reflect a draft synced from another device only while the user is not
		// typing — otherwise it would clobber an in-progress local edit (#62).
		if !m.chat.ComposerFocused() {
			m.chat.SetComposerValue(d.Draft)
		}

	case project.ChatTyping:
		return m.applyTypingLabelCmd(d.Typing)
	}
	return m, nil
}

// applyTypingLabel sets or clears the typing indicator without producing the
// animation commands, for the Reset path where the pane is being rebuilt.
func (m RootModel) applyTypingLabel(label string) {
	if label == "" {
		m.chat.ClearTypingLabel()
		return
	}
	m.chat.SetTypingLabel(label)
}

// applyTypingLabelCmd sets the typing indicator and starts the dot animation and
// the timeout that clears a label the peer never withdrew.
func (m RootModel) applyTypingLabelCmd(label string) (RootModel, tea.Cmd) {
	if label == "" {
		m.chat.ClearTypingLabel()
		return m, nil
	}
	alreadyActive := m.chat.IsTyping()
	m.typingSerial++
	serial := m.typingSerial
	m.chat.SetTypingLabel(label)
	cmds := []tea.Cmd{
		tea.Tick(6*time.Second, func(time.Time) tea.Msg { return clearTypingMsg{serial: serial} }),
	}
	if !alreadyActive {
		cmds = append(cmds, typingDotsTickCmd())
	}
	return m, tea.Batch(cmds...)
}

// clearChatBadgesOnOpen optimistically clears a chat's unread reactions and
// mentions when it is opened, and returns the commands that reconcile that with
// the server.
//
// TRANSITIONAL (#198): the optimistic half writes straight to the store rather
// than through the owner, so it bypasses the commit that would rebuild the
// projections. The reconciling commands come back through the owner and repaint.
func (m RootModel) clearChatBadgesOnOpen(chatID int64) (reactions, mentions tea.Cmd) {
	if m.st == nil {
		return nil, nil
	}
	c, ok := m.st.GetChat(chatID)
	if !ok {
		return nil, nil
	}
	if c.UnreadReactionsCount > 0 {
		m.st.SetChatReactionsRead(c.ID)
		reactions = m.readReactionsCmd(c.ID)
	}
	if c.UnreadMentionsCount > 0 {
		m.st.SetChatMentionsRead(c.ID)
		mentions = m.readMentionsCmd(c.ID)
	}
	if reactions != nil || mentions != nil {
		m.refreshProjections()
	}
	return reactions, mentions
}
