package domain_test

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/stretchr/testify/assert"
)

var (
	dmChat      = domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, IsContact: true}
	botChat     = domain.Chat{ID: 2, Peer: domain.Peer{ID: 2, Type: domain.PeerUser}, IsBot: true}
	groupChat   = domain.Chat{ID: 3, Peer: domain.Peer{ID: 3, Type: domain.PeerGroup}}
	channelChat = domain.Chat{ID: 4, Peer: domain.Peer{ID: 4, Type: domain.PeerChannel}}
	mutedDM     = domain.Chat{ID: 5, Peer: domain.Peer{ID: 5, Type: domain.PeerUser}, IsContact: true, IsMuted: true}
	unreadDM    = domain.Chat{ID: 6, Peer: domain.Peer{ID: 6, Type: domain.PeerUser}, IsContact: true, UnreadCount: 3}
)

func TestFolderFilter_ExcludePeer(t *testing.T) {
	f := domain.FolderFilter{Contacts: true, ExcludePeers: []int64{1}}
	assert.False(t, f.Matches(dmChat), "excluded peer must not match even if category matches")
}

func TestFolderFilter_IncludePeer_BypassesCategory(t *testing.T) {
	f := domain.FolderFilter{Groups: true, IncludePeers: []int64{1}}
	assert.True(t, f.Matches(dmChat), "explicitly included peer matches regardless of category flags")
}

func TestFolderFilter_PinnedPeer_BypassesCategory(t *testing.T) {
	f := domain.FolderFilter{Groups: true, PinnedPeers: []int64{1}}
	assert.True(t, f.Matches(dmChat))
}

func TestFolderFilter_IncludePeer_BypassesExclusionFlags(t *testing.T) {
	f := domain.FolderFilter{IncludePeers: []int64{5}, ExcludeMuted: true}
	assert.True(t, f.Matches(mutedDM), "explicitly included peer bypasses ExcludeMuted")
}

func TestFolderFilter_Contacts(t *testing.T) {
	f := domain.FolderFilter{Contacts: true}
	assert.True(t, f.Matches(dmChat))
	assert.False(t, f.Matches(groupChat))
}

func TestFolderFilter_NonContacts(t *testing.T) {
	nonContact := domain.Chat{ID: 7, Peer: domain.Peer{ID: 7, Type: domain.PeerUser}}
	f := domain.FolderFilter{NonContacts: true}
	assert.True(t, f.Matches(nonContact))
	assert.False(t, f.Matches(dmChat), "contact is not a non-contact")
	assert.False(t, f.Matches(botChat), "bot is not a non-contact")
}

func TestFolderFilter_Groups(t *testing.T) {
	f := domain.FolderFilter{Groups: true}
	assert.True(t, f.Matches(groupChat))
	assert.False(t, f.Matches(channelChat))
	assert.False(t, f.Matches(dmChat))
}

func TestFolderFilter_Broadcasts(t *testing.T) {
	f := domain.FolderFilter{Broadcasts: true}
	assert.True(t, f.Matches(channelChat))
	assert.False(t, f.Matches(groupChat))
}

func TestFolderFilter_Bots(t *testing.T) {
	f := domain.FolderFilter{Bots: true}
	assert.True(t, f.Matches(botChat))
	assert.False(t, f.Matches(dmChat))
}

func TestFolderFilter_ExcludeMuted(t *testing.T) {
	f := domain.FolderFilter{Contacts: true, ExcludeMuted: true}
	assert.True(t, f.Matches(dmChat))
	assert.False(t, f.Matches(mutedDM))
}

func TestFolderFilter_ExcludeRead(t *testing.T) {
	f := domain.FolderFilter{Contacts: true, ExcludeRead: true}
	assert.True(t, f.Matches(unreadDM))
	assert.False(t, f.Matches(dmChat), "read chat (UnreadCount==0) must be excluded")
}

func TestFolderFilter_SuperGroupMatchesGroups(t *testing.T) {
	superGroup := domain.Chat{ID: 8, Peer: domain.Peer{ID: 8, Type: domain.PeerSuperGroup}}
	f := domain.FolderFilter{Groups: true}
	assert.True(t, f.Matches(superGroup), "supergroup must match Groups flag")
}

func TestFolderFilter_SuperGroupDoesNotMatchBroadcasts(t *testing.T) {
	superGroup := domain.Chat{ID: 8, Peer: domain.Peer{ID: 8, Type: domain.PeerSuperGroup}}
	f := domain.FolderFilter{Broadcasts: true}
	assert.False(t, f.Matches(superGroup), "supergroup must not match Broadcasts flag")
}

func TestFolderFilter_NoFlagsNoMatches(t *testing.T) {
	f := domain.FolderFilter{ID: 1, Title: "Empty"}
	assert.False(t, f.Matches(dmChat))
	assert.False(t, f.Matches(groupChat))
}

func TestMatches_ExcludeArchived(t *testing.T) {
	f := domain.FolderFilter{Groups: true, ExcludeArchived: true}
	group := domain.Chat{ID: 1, Peer: domain.Peer{Type: domain.PeerGroup}}
	assert.True(t, f.Matches(group))

	archived := domain.Chat{ID: 2, Peer: domain.Peer{Type: domain.PeerGroup}, IsArchived: true}
	assert.False(t, f.Matches(archived))
}
