package ui

import (
	"testing"

	tg "github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestMediaBuilderFor_FileBuildsForcedDocument(t *testing.T) {
	att := &pendingAttachment{name: "report.pdf", mime: "application/pdf", sendAs: domain.MediaFile}
	build, ok := mediaBuilderFor(att)
	require.True(t, ok)
	media := build(&tg.InputFile{ID: 1})
	doc, ok := media.(*tg.InputMediaUploadedDocument)
	require.True(t, ok, "got %T, want *tg.InputMediaUploadedDocument", media)
	assert.True(t, doc.ForceFile)
	assert.Equal(t, "application/pdf", doc.MimeType)
	require.Len(t, doc.Attributes, 1)
	fn, ok := doc.Attributes[0].(*tg.DocumentAttributeFilename)
	require.True(t, ok)
	assert.Equal(t, "report.pdf", fn.FileName)
}

func TestMediaBuilderFor_PhotoStillSupported(t *testing.T) {
	att := &pendingAttachment{sendAs: domain.MediaPhoto}
	_, ok := mediaBuilderFor(att)
	assert.True(t, ok)
}

func TestMediaBuilderFor_VideoUnsupported(t *testing.T) {
	att := &pendingAttachment{sendAs: domain.MediaVideo}
	_, ok := mediaBuilderFor(att)
	assert.False(t, ok, "video send-as is #107, not yet supported")
}

func TestComputeFolderUnreads_ArchiveNoBadge_CountsArchivedInCustomFolder(t *testing.T) {
	st := store.NewMemory()
	// One archived unread group and one normal unread group in a group folder.
	st.SetChat(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerGroup}, IsArchived: true, UnreadCount: 2})
	st.SetChat(domain.Chat{ID: 2, Peer: domain.Peer{ID: 2, Type: domain.PeerGroup}, UnreadCount: 3})
	m := NewRootModel(nil, st, 50, false)
	m.folderBar.SetFolders([]domain.FolderFilter{{ID: 7, Title: "Groups", Groups: true}})
	m.folderBar.SetArchivePresent(true)

	counts := m.computeFolderUnreads()
	// Archive virtual folder carries no unread badge.
	_, hasArchive := counts[domain.ArchiveFolderID]
	assert.False(t, hasArchive)
	// Custom folders show archived chats in their listing, so the badge count
	// aligns with that content and includes the archived unread chat.
	assert.Equal(t, 2, counts[7])
}

func TestSyncFolderBar_TogglesArchivePresence(t *testing.T) {
	st := store.NewMemory()
	st.SetChat(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})
	m := NewRootModel(nil, st, 50, false)

	m.syncFolderBar()
	for _, f := range m.folderBar.Folders() {
		require.NotEqual(t, domain.ArchiveFolderID, f.ID)
	}

	st.SetChatArchived(1, true)
	m.syncFolderBar()
	last := m.folderBar.Folders()
	assert.Equal(t, domain.ArchiveFolderID, last[len(last)-1].ID)
}

func TestFilteredChats_ArchiveSplit(t *testing.T) {
	st := store.NewMemory()
	st.SetChat(domain.Chat{ID: 1, Title: "Normal", Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})
	st.SetChat(domain.Chat{ID: 2, Title: "Archived", Peer: domain.Peer{ID: 2, Type: domain.PeerUser}, IsArchived: true})

	m := NewRootModel(nil, st, 50, false)

	// All Chats (nil filter): archived hidden.
	m.activeFilter = nil
	got := m.filteredChats()
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].ID)

	// Archive virtual folder: only archived.
	arch := domain.FolderFilter{ID: domain.ArchiveFolderID, Title: "Archive"}
	m.activeFilter = &arch
	got = m.filteredChats()
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].ID)
}

func TestFilteredChats_CustomFolderIncludesArchivedExplicitPeer(t *testing.T) {
	st := store.NewMemory()
	st.SetChat(domain.Chat{ID: 1, Title: "Normal", Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})
	st.SetChat(domain.Chat{ID: 2, Title: "Archived", Peer: domain.Peer{ID: 2, Type: domain.PeerChannel}, IsArchived: true})

	m := NewRootModel(nil, st, 50, false)
	f := domain.FolderFilter{ID: 7, Title: "News", IncludePeers: []int64{2}}
	m.activeFilter = &f

	got := m.filteredChats()
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].ID)
}

func TestFilteredChats_CustomFolderIncludesArchivedCategoryMatch(t *testing.T) {
	st := store.NewMemory()
	st.SetChat(domain.Chat{ID: 1, Title: "Normal", Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})
	st.SetChat(domain.Chat{ID: 2, Title: "Archived", Peer: domain.Peer{ID: 2, Type: domain.PeerGroup}, IsArchived: true})

	m := NewRootModel(nil, st, 50, false)
	f := domain.FolderFilter{ID: 7, Title: "Groups", Groups: true}
	m.activeFilter = &f

	got := m.filteredChats()
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].ID)
}
