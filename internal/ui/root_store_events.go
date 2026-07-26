package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// handleChange renders an applied domain change. It performs no mutation: the
// store is already up to date by the time this runs.
func (m RootModel) handleChange(chg state.Change) (RootModel, tea.Cmd) {
	if m.st == nil {
		return m, nil
	}
	switch chg.Kind {
	case state.ChangeNewMessage:
		incomingOther := chg.ChatID != m.currentChatID && !chg.Message.IsOut
		m.chatList.SetChats(m.filteredChats())
		// Folder unread counts only depend on per-chat unread; recompute solely
		// when this message actually bumped a chat's unread count.
		if m.folderBar != nil && chg.UnreadChanged {
			m.syncFolderBar()
		}
		// Flash the row of a non-open chat that just bumped to the top so the eye
		// can follow the reorder (issue #39, second section).
		var highlightCmd tea.Cmd
		if incomingOther {
			m.chatList.HighlightChat(chg.ChatID)
			m.chatHighlightSerial++
			highlightCmd = chatHighlightFadeCmd(m.chatHighlightSerial)
		}
		// In-app notification: a fresh message in an inactive, unmuted chat pops a
		// top-right toast (#59), gated identically to OS notifications.
		var notifyCmd tea.Cmd
		if incomingOther && store.Notifiable(m.st, chg.ChatID, m.currentChatID, chg.Message.Date, time.Now()) {
			notifyCmd = m.showInAppNotify(chg.Message)
		}
		if chg.ChatID == m.currentChatID {
			m.chat.SetMessages(m.st.Messages(m.currentChatID))
			cmds := []tea.Cmd{m.markReadCmd(), m.pendingDownloadCmds([]store.Message{chg.Message})}
			if m.focus == FocusChat && chg.Message.Mentioned {
				cmds = append(cmds, m.readMentionsCmd(chg.ChatID))
			}
			return m, tea.Batch(cmds...)
		}
		return m, tea.Batch(highlightCmd, notifyCmd)

	case state.ChangeReadInbox:
		if chat, ok := m.st.GetChat(chg.ChatID); ok {
			m.chatList.SetChatUnread(chg.ChatID, chat.UnreadCount)
		}
		if m.folderBar != nil {
			m.syncFolderBar()
		}

	case state.ChangeReadOutbox:
		if chg.ChatID == m.currentChatID {
			if chat, ok := m.st.GetChat(chg.ChatID); ok {
				m.chat.SetOutboxReadMaxID(chat.ReadOutboxMaxID)
			}
		}

	case state.ChangeMessageEdited:
		// A message was edited on another client: the stored text and edit date
		// are already updated, so re-render the open chat in place (no history
		// reload) and keep the scroll position.
		if chg.ChatID == m.currentChatID {
			m.chat.SetMessagesKeepScroll(m.st.Messages(m.currentChatID))
		}

	case state.ChangeMessageReactions:
		if chg.ChatID == m.currentChatID {
			m.chat.SetMessagesKeepScroll(m.st.Messages(m.currentChatID))
			if m.focus == FocusChat && chg.ReactionsUnread {
				return m, m.readReactionsCmd(chg.ChatID)
			}
			return m, nil
		}
		if chg.UnreadReactionChanged {
			m.chatList.SetChats(m.filteredChats())
		}

	case state.ChangeDraft:
		// Reflect a synced draft live only when this chat is open and the user is
		// not actively typing — otherwise we would clobber an in-progress local
		// edit. For other chats, seed the session cache without overwriting a
		// newer unsent local draft (#62).
		if chg.ChatID == m.currentChatID {
			if !m.chat.ComposerFocused() {
				m.chat.SetComposerValue(chg.Draft)
			}
		} else {
			m.chat.SeedDraft(chg.ChatID, chg.Draft)
		}

	case state.ChangeMessagesDeleted:
		if chg.ChatID == 0 || chg.ChatID == m.currentChatID {
			m.chat.SetMessages(m.st.Messages(m.currentChatID))
		}
		m.chatList.SetChats(m.filteredChats())

	case state.ChangePresence:
		m.chatList.SetChats(m.filteredChats())
		if chg.ChatID == m.currentChatID {
			if chat, ok := m.st.GetChat(chg.ChatID); ok {
				m.chat.SetChat(&chat)
			}
		}

	case state.ChangeMute:
		m.chatList.SetChats(m.filteredChats())
		if m.folderBar != nil {
			m.syncFolderBar()
		}
		if chg.ChatID == m.currentChatID {
			if c, ok := m.st.GetChat(chg.ChatID); ok {
				m.chat.SetChat(&c)
			}
		}

	case state.ChangeTyping:
		if chg.ChatID != m.currentChatID {
			return m, nil
		}
		label := chg.Typing.Label()
		if label == "" {
			m.chat.ClearTypingLabel()
			return m, nil
		}
		alreadyActive := m.chat.IsTyping()
		m.typingSerial++
		serial := m.typingSerial
		m.chat.SetTypingLabel(label)
		var cmds []tea.Cmd
		cmds = append(cmds, tea.Tick(6*time.Second, func(time.Time) tea.Msg { return clearTypingMsg{serial: serial} }))
		if !alreadyActive {
			cmds = append(cmds, typingDotsTickCmd())
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// notifyOpenMsg is emitted when a notify toast is clicked: it dismisses the
// toast and opens the target chat (#59 click-to-open).
type notifyOpenMsg struct {
	chat   store.Chat
	serial int
}

// showInAppNotify adds a top-right notify toast for an incoming message in an
// inactive chat and returns its auto-dismiss command. The whole toast is a
// click target that opens the chat. Respects the notification-preview setting.
func (m RootModel) showInAppNotify(msg store.Message) tea.Cmd {
	chat, _ := m.st.GetChat(msg.ChatID)
	title := "New message"
	if chat.Title != "" {
		title = chat.Title
	}
	body := "New message"
	if m.cfg == nil || m.cfg.UI.NotificationPreview {
		body = truncatePreview(msg.Text, 100)
	}
	serial := m.toasts.Add(components.ToastNotify, title+"\n"+body)
	m.toasts.SetClick(serial, notifyOpenMsg{chat: chat, serial: serial})
	return tea.Tick(durationFor(components.SeverityInfo), func(time.Time) tea.Msg {
		return ClearStatusErrMsg{Serial: serial}
	})
}

// truncatePreview shortens s to at most n runes, appending an ellipsis when cut.
func truncatePreview(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
