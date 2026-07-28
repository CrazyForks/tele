package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// handleChatMenuRequest applies an optimistic store mutation for a
// chat-menu action, refreshes the visible panes, and returns the async
// Telegram command. The returned bool is false when msg is not a
// chat-menu request.
func (m RootModel) handleChatMenuRequest(msg tea.Msg) (RootModel, tea.Cmd, bool) {
	switch req := msg.(type) {
	case components.ToggleMuteRequest:
		m.chatMenu = nil
		if m.st != nil {
			m.st.SetChatMuted(req.Peer.ID, req.Muted)
		}
		m.refreshProjections()
		peer, muted := req.Peer, req.Muted
		return m, func() tea.Msg { _ = m.tgClient.SetMuted(m.ctx, peer, muted); return nil }, true

	case components.ToggleUnreadRequest:
		m.chatMenu = nil
		peer := req.Peer
		if req.Unread {
			if m.st != nil {
				m.st.SetChatUnreadMark(peer.ID, true)
			}
			m.refreshProjections()
			return m, func() tea.Msg { _ = m.tgClient.MarkDialogUnread(m.ctx, peer, true); return nil }, true
		}
		// Mark as read: clear manual mark and unread count, read to top.
		if m.st != nil {
			if c, ok := m.st.GetChat(peer.ID); ok {
				c.UnreadMark = false
				c.UnreadCount = 0
				m.st.SetChat(c)
			}
		}
		m.refreshProjections()
		return m, func() tea.Msg { _ = m.tgClient.MarkRead(m.ctx, peer, 0); return nil }, true

	case components.AddToFolderRequest:
		m.chatMenu = nil
		if m.st != nil {
			filters := m.st.FolderFilters()
			for i := range filters {
				if filters[i].ID == req.FilterID {
					filters[i].IncludePeers = toggleInt64(filters[i].IncludePeers, req.Peer.ID, req.Add)
				}
			}
			m.st.SetFolderFilters(filters)
		}
		m.refreshProjections()
		peer, filterID, add := req.Peer, req.FilterID, req.Add
		return m, func() tea.Msg { _ = m.tgClient.AddToFolder(m.ctx, filterID, peer, add); return nil }, true

	case components.ToggleArchiveRequest:
		m.chatMenu = nil
		if m.st != nil {
			m.st.SetChatArchived(req.Peer.ID, req.Archived)
		}
		m.refreshProjections()
		peer, archived := req.Peer, req.Archived
		return m, func() tea.Msg { _ = m.tgClient.SetArchived(m.ctx, peer, archived); return nil }, true
	}
	return m, nil, false
}

// refreshChatPanes is gone: an optimistic store write commits, the commit
// rebuilds the subscribed projections, and the resulting delta repaints the
// panes. Nothing has to remember to refresh.

func toggleInt64(ids []int64, id int64, add bool) []int64 {
	out := make([]int64, 0, len(ids)+1)
	found := false
	for _, e := range ids {
		if e == id {
			found = true
			if !add {
				continue
			}
		}
		out = append(out, e)
	}
	if add && !found {
		out = append(out, id)
	}
	return out
}
