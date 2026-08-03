package core

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/telerr"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
)

// stubClient records what a command asked Telegram to do and answers with err.
// The embedded interface is nil, so a command reaching for anything this stub
// does not declare panics rather than passing quietly.
type stubClient struct {
	internaltg.Client
	err error

	mutedWith    *bool
	archivedWith *bool
	unreadWith   *bool
	readTo       int
	editedTo     string
	deletedIDs   []int
	revoked      bool
	reactedWith  string
	reactionSent bool
	forwardedTo  int64
	forwardedIDs []int
	sentText     string
	typingCalls  int
	draftText    string
	searchedFor  string
	searchLimit  int

	// Send bookkeeping for the outbox worker (#193). Guarded because the worker
	// calls from its own goroutine while the test asserts from another.
	sendMu       sync.Mutex
	sendCount    int
	sentRandomID int64
	sentID       int
	// sendBlock, when set, holds SendMessage open so a test can catch an entry
	// mid-flight and drop the owner under it.
	sendBlock chan struct{}
}

func (s *stubClient) Connect(context.Context, *config.Config, *internaltg.AuthFlow, chan<- struct{}, func(int64, string)) error {
	return nil
}

func (s *stubClient) Updates() <-chan store.Event { return nil }

func (s *stubClient) SetMuted(_ context.Context, _ domain.Peer, muted bool) error {
	s.mutedWith = &muted
	return s.err
}

func (s *stubClient) SetArchived(_ context.Context, _ domain.Peer, archived bool) error {
	s.archivedWith = &archived
	return s.err
}

func (s *stubClient) MarkDialogUnread(_ context.Context, _ domain.Peer, unread bool) error {
	s.unreadWith = &unread
	return s.err
}

func (s *stubClient) AddToFolder(_ context.Context, _ int, _ domain.Peer, _ bool) error {
	return s.err
}

func (s *stubClient) MarkRead(_ context.Context, _ domain.Peer, maxID int) error {
	s.readTo = maxID
	return s.err
}

func (s *stubClient) ReadReactions(_ context.Context, _ domain.Peer) error { return s.err }

func (s *stubClient) ReadMentions(_ context.Context, _ domain.Peer) error { return s.err }

// newCmdOwner builds an owner over the stub, with one chat already known.
func newCmdOwner(t *testing.T, c *stubClient) (*Owner, store.Store) {
	t.Helper()
	o, s := newOwnerWithClient(t, c)
	st := s.Store()
	st.SetChat(domain.Chat{ID: 1, Title: "Ada", Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})
	return o, st
}

func TestSetMuted_AppliesOptimisticallyAndKeepsItOnSuccess(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)

	require.NoError(t, o.SetMuted(context.Background(), 1, true))

	chat, _ := st.GetChat(1)
	assert.True(t, chat.IsMuted)
	require.NotNil(t, c.mutedWith)
	assert.True(t, *c.mutedWith)
}

func TestSetMuted_RollsBackAndReturnsTheError(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Forbidden}}
	o, st := newCmdOwner(t, c)

	err := o.SetMuted(context.Background(), 1, true)

	require.Error(t, err)
	assert.Equal(t, telerr.Forbidden, telerr.Of(err))
	chat, _ := st.GetChat(1)
	assert.False(t, chat.IsMuted, "a failed mute must not stay on screen")
}

func TestSetMuted_UnknownChatIsPeerNotFound(t *testing.T) {
	o, _ := newCmdOwner(t, &stubClient{})

	err := o.SetMuted(context.Background(), 99, true)

	assert.Equal(t, telerr.PeerNotFound, telerr.Of(err))
}

func TestSetMuted_PublishesADeltaToTheChatList(t *testing.T) {
	o, _ := newCmdOwner(t, &stubClient{})
	o.Subscribe(project.ChatListWindow{Limit: 10})
	recvDelta(t, o.Deltas()) // the subscription's opening Reset

	require.NoError(t, o.SetMuted(context.Background(), 1, true))

	d, ok := recvDelta(t, o.Deltas())
	require.True(t, ok, "the mute must reach subscribed clients")
	require.NotNil(t, d.ChatList)
}

func TestSetArchived_KeptOnSuccess(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)

	require.NoError(t, o.SetArchived(context.Background(), 1, true))

	chat, _ := st.GetChat(1)
	assert.True(t, chat.IsArchived)
	require.NotNil(t, c.archivedWith)
	assert.True(t, *c.archivedWith)
}

func TestSetArchived_RollsBackOnFailure(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network}}
	o, st := newCmdOwner(t, c)

	err := o.SetArchived(context.Background(), 1, true)

	require.Error(t, err)
	chat, _ := st.GetChat(1)
	assert.False(t, chat.IsArchived, "a refused archive must not stay on screen")
}

func TestSetUnreadMark_KeptOnSuccess(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)

	require.NoError(t, o.SetUnreadMark(context.Background(), 1, true))

	chat, _ := st.GetChat(1)
	assert.True(t, chat.UnreadMark)
	require.NotNil(t, c.unreadWith)
	assert.True(t, *c.unreadWith)
}

func TestSetUnreadMark_RollsBackOnFailure(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network}}
	o, st := newCmdOwner(t, c)

	err := o.SetUnreadMark(context.Background(), 1, true)

	require.Error(t, err)
	chat, _ := st.GetChat(1)
	assert.False(t, chat.UnreadMark, "a refused mark must not stay on screen")
}

func TestAddToFolder_KeepsTheMembershipOnSuccess(t *testing.T) {
	o, st := newCmdOwner(t, &stubClient{})
	st.SetFolderFilters([]domain.FolderFilter{{ID: 3, Title: "Work"}})

	require.NoError(t, o.AddToFolder(context.Background(), 3, 1, true))

	filters := st.FolderFilters()
	require.Len(t, filters, 1)
	assert.Equal(t, []int64{1}, filters[0].IncludePeers)
}

func TestAddToFolder_RollsBackTheMembershipOnFailure(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Forbidden}}
	o, st := newCmdOwner(t, c)
	st.SetFolderFilters([]domain.FolderFilter{{ID: 3, Title: "Work"}})

	err := o.AddToFolder(context.Background(), 3, 1, true)

	require.Error(t, err)
	filters := st.FolderFilters()
	require.Len(t, filters, 1)
	assert.Empty(t, filters[0].IncludePeers, "a refused add must not leave the chat in the folder")
}

func TestAddToFolder_UnknownFilterIsANoOp(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)
	st.SetFolderFilters([]domain.FolderFilter{{ID: 3, Title: "Work"}})

	require.NoError(t, o.AddToFolder(context.Background(), 99, 1, true))

	assert.Empty(t, st.FolderFilters()[0].IncludePeers)
}

// The read pointer is not optimistic: it moves only once Telegram confirms,
// because an unread count that ran ahead of the server would lie.
func TestMarkRead_MovesThePointerOnlyAfterConfirmation(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network}}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, ReadInboxMaxID: 10})

	err := o.MarkRead(context.Background(), 1, 20)

	require.Error(t, err)
	chat, _ := st.GetChat(1)
	assert.Equal(t, 10, chat.ReadInboxMaxID, "a refused mark-read must not move the pointer")
}

func TestMarkRead_AdvancesThePointerOnSuccess(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, ReadInboxMaxID: 10})

	require.NoError(t, o.MarkRead(context.Background(), 1, 20))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 20, chat.ReadInboxMaxID)
	assert.Equal(t, 20, c.readTo)
}

// The chat menu reads a whole chat: maxID 0 means "everything", not an advance
// to message zero, so the badge clears outright.
func TestMarkRead_ZeroMaxIDClearsTheWholeChat(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser},
		ReadInboxMaxID: 10, UnreadCount: 7, UnreadMark: true,
	})

	require.NoError(t, o.MarkRead(context.Background(), 1, 0))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 0, chat.UnreadCount)
	assert.False(t, chat.UnreadMark, "reading a chat also clears the manual mark")
	assert.Equal(t, 0, c.readTo)
}

func TestReadReactions_ClearsTheBadgeOnSuccess(t *testing.T) {
	o, st := newCmdOwner(t, &stubClient{})
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, UnreadReactionsCount: 2,
	})

	require.NoError(t, o.ReadReactions(context.Background(), 1))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 0, chat.UnreadReactionsCount)
}

// The badge clears up front so an opened chat drops its indicators at once
// (#142, #155); a failure is reported but does not relight it, since the count
// cannot be reconstructed and the next dialog-list sync is authoritative.
func TestReadReactions_ClearsTheBadgeBeforeTheRequest(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network}}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, UnreadReactionsCount: 2,
	})

	require.Error(t, o.ReadReactions(context.Background(), 1))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 0, chat.UnreadReactionsCount)
}

func TestReadMentions_ClearsTheBadgeOnSuccess(t *testing.T) {
	o, st := newCmdOwner(t, &stubClient{})
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, UnreadMentionsCount: 3,
	})

	require.NoError(t, o.ReadMentions(context.Background(), 1))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 0, chat.UnreadMentionsCount)
}

func TestReadMentions_ClearsTheBadgeBeforeTheRequest(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network}}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}, UnreadMentionsCount: 3,
	})

	require.Error(t, o.ReadMentions(context.Background(), 1))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 0, chat.UnreadMentionsCount)
}

// Reading a whole chat must move the read pointer too, not just clear the
// count: the "New messages" divider is drawn from the pointer, so a chat marked
// read from the menu would still open with a divider.
func TestMarkRead_ZeroMaxIDMovesThePointerToTheNewestMessage(t *testing.T) {
	c := &stubClient{}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{
		ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser},
		ReadInboxMaxID: 10, UnreadCount: 3,
		LastMessage: &domain.Message{ID: 42, ChatID: 1},
	})

	require.NoError(t, o.MarkRead(context.Background(), 1, 0))

	chat, _ := st.GetChat(1)
	assert.Equal(t, 42, chat.ReadInboxMaxID, "everything up to the newest message is read")
	assert.Equal(t, 0, chat.UnreadCount)
}
