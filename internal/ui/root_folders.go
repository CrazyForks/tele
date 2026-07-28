package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

// domainArchiveFolderID is the Archive virtual folder, named here so the folder
// handling reads without a domain import at every use.
const domainArchiveFolderID = domain.ArchiveFolderID

// selectFolder re-points the chatlist window at another folder. Filtering and
// ordering are the core's: the client only says which folder it is looking at.
func (m RootModel) selectFolder(id int) (RootModel, tea.Cmd) {
	m.activeFolder = id
	if m.owner != nil && m.chatListSub != 0 {
		offset, limit, _ := m.chatList.WindowRequest()
		m.owner.MoveWindow(m.chatListSub, project.ChatListWindow{
			Folder: id,
			Offset: offset,
			Limit:  limit,
		})
	}
	return m, nil
}
