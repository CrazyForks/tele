package ui

import (
	"context"

	"github.com/sorokin-vladimir/tele/internal/core/project"
)

// Owner is the client's view of the connection owner: the subscription surface
// and nothing else. The UI holds this rather than *core.Owner so it can be
// driven by a double in tests, and so the day the owner moves behind a socket
// the client keeps the same interface.
type Owner interface {
	Subscribe(w project.Window) project.SubID
	MoveWindow(id project.SubID, w project.Window)
	Unsubscribe(id project.SubID)
	// Refresh rebuilds the subscriptions after an optimistic store write that
	// bypassed the owner. TRANSITIONAL (#193, #195, #196, #198).
	Refresh()

	// Commands. Each applies its own optimistic change and undoes it if
	// Telegram refuses, so the client only decides how a failure looks.
	SetMuted(ctx context.Context, chatID int64, muted bool) error
	SetArchived(ctx context.Context, chatID int64, archived bool) error
	SetUnreadMark(ctx context.Context, chatID int64, unread bool) error
	AddToFolder(ctx context.Context, filterID int, chatID int64, add bool) error
	// MarkRead with maxID 0 reads the whole chat.
	MarkRead(ctx context.Context, chatID int64, maxID int) error
	ReadReactions(ctx context.Context, chatID int64) error
	ReadMentions(ctx context.Context, chatID int64) error
}

// refreshProjections repaints after an optimistic write the client made
// directly. TRANSITIONAL (#198): when commands go through the owner, its commit
// does this and every call site here disappears.
func (m *RootModel) refreshProjections() {
	if m.owner != nil {
		m.owner.Refresh()
	}
}

// WithOwner attaches the owner the model subscribes to.
func (m RootModel) WithOwner(o Owner) RootModel {
	m.owner = o
	return m
}

// subscribeChatList opens (or re-opens) the chatlist subscription for the
// current folder and the window the list component wants.
func (m *RootModel) subscribeChatList() {
	if m.owner == nil {
		return
	}
	if m.chatListSub != 0 {
		m.owner.Unsubscribe(m.chatListSub)
	}
	offset, limit, _ := m.chatList.WindowRequest()
	m.chatListSub = m.owner.Subscribe(project.ChatListWindow{
		Folder: m.activeFolder,
		Offset: offset,
		Limit:  limit,
	})
}

// syncChatListWindow asks the owner for a wider or shifted window when the
// cursor has moved near the edge of the one the client holds. It is a no-op
// while the cursor stays inside the overscan, which is the common case.
func (m *RootModel) syncChatListWindow() {
	if m.owner == nil || m.chatListSub == 0 {
		return
	}
	offset, limit, changed := m.chatList.WindowRequest()
	if !changed {
		return
	}
	m.owner.MoveWindow(m.chatListSub, project.ChatListWindow{
		Folder: m.activeFolder,
		Offset: offset,
		Limit:  limit,
	})
}
