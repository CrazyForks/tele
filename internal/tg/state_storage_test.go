package tg_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gotd/td/telegram/updates"
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

func TestSQLiteState_SetDateSeq_UpdatesBoth(t *testing.T) {
	ss := newTestStateStorage(t)
	ctx := context.Background()
	require.NoError(t, ss.SetState(ctx, 1, updates.State{}))
	require.NoError(t, ss.SetDateSeq(ctx, 1, 777, 888))
	got, _, _ := ss.GetState(ctx, 1)
	assert.Equal(t, 777, got.Date)
	assert.Equal(t, 888, got.Seq)
}
