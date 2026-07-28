package tg_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gotd/td/telegram/updates"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
)

func newTestStateStorage(t *testing.T) updates.StateStorage {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "state.db"), zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return internaltg.NewSQLiteStateStorage(s.DB())
}

func TestSQLiteState_GetState_MissingReturnsNotFound(t *testing.T) {
	ss := newTestStateStorage(t)
	_, found, err := ss.GetState(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSQLiteState_SetState_GetState_RoundTrip(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	want := updates.State{Pts: 100, Qts: 200, Date: 300, Seq: 400}
	require.NoError(t, ss.SetState(ctx, 1, want))
	got, found, err := ss.GetState(ctx, 1)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, want, got)
}

func TestSQLiteState_SetPts_UpdatesField(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	require.NoError(t, ss.SetState(ctx, 1, updates.State{Pts: 1}))
	require.NoError(t, ss.SetPts(ctx, 1, 999))
	got, _, _ := ss.GetState(ctx, 1)
	assert.Equal(t, 999, got.Pts)
}

func TestSQLiteState_SetPts_ErrorWhenNoState(t *testing.T) {
	ss := newTestStateStorage(t)
	err := ss.SetPts(context.Background(), 1, 10)
	assert.Error(t, err)
}

func TestSQLiteState_ChannelPts_RoundTrip(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	require.NoError(t, ss.SetChannelPts(ctx, 1, 42, 555))
	pts, found, err := ss.GetChannelPts(ctx, 1, 42)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 555, pts)
}

func TestSQLiteState_GetChannelPts_MissingReturnsNotFound(t *testing.T) {
	ss := newTestStateStorage(t)
	_, found, err := ss.GetChannelPts(context.Background(), 1, 99)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSQLiteState_ForEachChannels_VisitsAll(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	require.NoError(t, ss.SetChannelPts(ctx, 1, 10, 100))
	require.NoError(t, ss.SetChannelPts(ctx, 1, 20, 200))
	seen := map[int64]int{}
	err := ss.ForEachChannels(ctx, 1, func(_ context.Context, channelID int64, pts int) error {
		seen[channelID] = pts
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{10: 100, 20: 200}, seen)
}

func TestSQLiteState_ChannelAccessHash_RoundTrip(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	// The storage must also satisfy updates.ChannelAccessHasher so the manager
	// persists channel access hashes and re-registers channels at startup (#119).
	h, ok := ss.(updates.ChannelAccessHasher)
	require.True(t, ok, "state storage must implement updates.ChannelAccessHasher")

	_, found, err := h.GetChannelAccessHash(ctx, 1, 42)
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, h.SetChannelAccessHash(ctx, 1, 42, 0x7fffffffffffffff))
	hash, found, err := h.GetChannelAccessHash(ctx, 1, 42)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, int64(0x7fffffffffffffff), hash)

	// Overwrite (gotd uses INSERT OR REPLACE semantics on re-learn).
	require.NoError(t, h.SetChannelAccessHash(ctx, 1, 42, 123))
	hash, _, err = h.GetChannelAccessHash(ctx, 1, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(123), hash)
}

// TestSQLiteState_ForEachChannels_NestedQuery guards against a deadlock: gotd's
// manager startup calls GetChannelAccessHash from inside the ForEachChannels
// callback. With the pool pinned to one connection (SetMaxOpenConns(1)), holding
// the cursor open across that nested query deadlocks (#119 follow-up). The
// timeout context fails the test instead of hanging the suite if it regresses.
func TestSQLiteState_ForEachChannels_NestedQuery(t *testing.T) {
	ss := newTestStateStorage(t)
	h := ss.(updates.ChannelAccessHasher)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, ss.SetChannelPts(ctx, 1, 10, 100))
	require.NoError(t, ss.SetChannelPts(ctx, 1, 20, 200))
	require.NoError(t, h.SetChannelAccessHash(ctx, 1, 10, 1010))

	seen := map[int64]int64{}
	err := ss.ForEachChannels(ctx, 1, func(ctx context.Context, channelID int64, _ int) error {
		hash, _, err := h.GetChannelAccessHash(ctx, 1, channelID)
		if err != nil {
			return err
		}
		seen[channelID] = hash
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{10: 1010, 20: 0}, seen)
}

func TestSQLiteState_GetChannelAccessHash_FallsBackToChatStore(t *testing.T) {
	st, err := store.NewSQLite(filepath.Join(t.TempDir(), "state.db"), zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	h := internaltg.NewSQLiteStateStorage(st.DB()).(updates.ChannelAccessHasher)
	ctx := context.Background()

	// Channel known to the chat store (from GetDialogs) but absent from the
	// dedicated channel_access_hash table — the #119 startup gap.
	st.SetChat(domain.Chat{ID: 555, Peer: domain.Peer{ID: 555, Type: domain.PeerChannel, AccessHash: 9999}})
	st.SetChat(domain.Chat{ID: 666, Peer: domain.Peer{ID: 666, Type: domain.PeerSuperGroup, AccessHash: 8888}})

	for id, want := range map[int64]int64{555: 9999, 666: 8888} {
		hash, found, err := h.GetChannelAccessHash(ctx, 1, id)
		require.NoError(t, err)
		assert.True(t, found, "channel %d should resolve via chat-store fallback", id)
		assert.Equal(t, want, hash)
	}

	// The dedicated table takes precedence once populated.
	require.NoError(t, h.SetChannelAccessHash(ctx, 1, 555, 1234))
	hash, _, err := h.GetChannelAccessHash(ctx, 1, 555)
	require.NoError(t, err)
	assert.Equal(t, int64(1234), hash)

	// A non-channel peer (user) must not match the fallback.
	st.SetChat(domain.Chat{ID: 777, Peer: domain.Peer{ID: 777, Type: domain.PeerUser, AccessHash: 4242}})
	_, found, err := h.GetChannelAccessHash(ctx, 1, 777)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestSQLiteState_SetDateSeq_UpdatesBoth(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	require.NoError(t, ss.SetState(ctx, 1, updates.State{}))
	require.NoError(t, ss.SetDateSeq(ctx, 1, 777, 888))
	got, _, _ := ss.GetState(ctx, 1)
	assert.Equal(t, 777, got.Date)
	assert.Equal(t, 888, got.Seq)
}
