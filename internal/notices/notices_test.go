package notices_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/notices"
)

func TestPending_ReturnsOnlyUnseenInOrder(t *testing.T) {
	seen := notices.NewMemorySeen()
	seen.MarkSeen("b")

	all := []notices.Notice{
		{ID: "a", Title: "A", Body: "body a", Delay: time.Second},
		{ID: "b", Title: "B", Body: "body b", Delay: time.Second},
		{ID: "c", Title: "C", Body: "body c", Delay: time.Second},
	}

	got := notices.Pending(all, seen)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "c", got[1].ID)
}

func TestPending_EmptyWhenAllSeen(t *testing.T) {
	seen := notices.NewMemorySeen()
	seen.MarkSeen("a")

	got := notices.Pending([]notices.Notice{{ID: "a"}}, seen)
	assert.Empty(t, got)
}

func TestMemorySeen_RoundTrips(t *testing.T) {
	seen := notices.NewMemorySeen()
	assert.False(t, seen.IsSeen("x"))
	seen.MarkSeen("x")
	assert.True(t, seen.IsSeen("x"))
}
