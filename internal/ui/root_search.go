package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// handleSearchUsers runs a server-side contact search for the search overlay
// and routes the outcome back to the model as a SearchUsersResult (#82).
func (m RootModel) handleSearchUsers(req screens.SearchUsersRequest) (RootModel, tea.Cmd) {
	if m.owner == nil {
		return m, nil
	}
	ctx, owner := m.ctx, m.owner
	query, serial := req.Query, req.Serial
	const limit = 10
	return m, func() tea.Msg {
		chats, err := owner.SearchContacts(ctx, query, limit)
		return screens.SearchUsersResult{Query: query, Serial: serial, Chats: chats, Err: err}
	}
}
