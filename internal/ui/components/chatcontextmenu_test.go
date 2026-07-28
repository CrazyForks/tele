package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
)

func TestChatMenu_ReadLabelForUnreadChat(t *testing.T) {
	chat := domain.Chat{ID: 1, UnreadCount: 3, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}}
	cm := components.NewChatContextMenu(chat, nil, keys.DefaultKeyMap())
	assert.Contains(t, cm.View(), "Mark as read")
}

func TestChatMenu_UnreadLabelForReadChat(t *testing.T) {
	chat := domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}}
	cm := components.NewChatContextMenu(chat, nil, keys.DefaultKeyMap())
	assert.Contains(t, cm.View(), "Mark as unread")
}

func TestChatMenu_MuteToggleLabel(t *testing.T) {
	muted := components.NewChatContextMenu(domain.Chat{ID: 1, IsMuted: true}, nil, keys.DefaultKeyMap())
	assert.Contains(t, muted.View(), "Unmute")

	unmuted := components.NewChatContextMenu(domain.Chat{ID: 1}, nil, keys.DefaultKeyMap())
	v := unmuted.View()
	assert.Contains(t, v, "Mute")
	assert.NotContains(t, v, "Unmute")
}

func TestChatMenu_EmitsToggleMuteRequest(t *testing.T) {
	chat := domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}}
	cm := components.NewChatContextMenu(chat, nil, keys.DefaultKeyMap())
	_, cmd := cm.Update(keyMsg('m')) // direct key -> Mute
	require.NotNil(t, cmd)
	req, ok := cmd().(components.ToggleMuteRequest)
	require.True(t, ok)
	assert.True(t, req.Muted)
	assert.Equal(t, int64(1), req.Peer.ID)
}

func TestChatMenu_FolderSubmenuToggle(t *testing.T) {
	folders := []domain.FolderFilter{{ID: 7, Title: "Work", IncludePeers: []int64{1}}}
	chat := domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}}
	cm := components.NewChatContextMenu(chat, folders, keys.DefaultKeyMap())

	// open the folder submenu via direct key 'f'
	_, cmd := cm.Update(keyMsg('f'))
	require.Nil(t, cmd) // opening a submenu does not emit
	assert.Contains(t, cm.View(), "✓ Work")

	// cursor starts at the first folder; Enter toggles membership (remove).
	_, cmd = cm.Update(pressEnter())
	require.NotNil(t, cmd)
	req, ok := cmd().(components.AddToFolderRequest)
	require.True(t, ok)
	assert.Equal(t, 7, req.FilterID)
	assert.False(t, req.Add)
}

func TestChatMenu_ArchiveEntry(t *testing.T) {
	cm := components.NewChatContextMenu(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1}}, nil, keys.DefaultKeyMap())
	assert.Contains(t, cm.View(), "Archive")

	cmA := components.NewChatContextMenu(domain.Chat{ID: 1, IsArchived: true, Peer: domain.Peer{ID: 1}}, nil, keys.DefaultKeyMap())
	assert.Contains(t, cmA.View(), "Unarchive")
}

func TestChatMenu_EmitsToggleArchiveRequest(t *testing.T) {
	cm := components.NewChatContextMenu(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}}, nil, keys.DefaultKeyMap())
	_, cmd := cm.Update(keyMsg('a')) // direct key -> Archive
	require.NotNil(t, cmd)
	req, ok := cmd().(components.ToggleArchiveRequest)
	require.True(t, ok)
	assert.True(t, req.Archived)
}
