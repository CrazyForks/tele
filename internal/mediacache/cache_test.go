package mediacache_test

import (
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/mediacache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_PutGetRoundTrip(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	c.Put("a", []byte("hello"))
	got, ok := c.Get("a")

	require.True(t, ok)
	assert.Equal(t, []byte("hello"), got)
}

func TestCache_MissReturnsFalse(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	_, ok := c.Get("absent")
	assert.False(t, ok)
}

func TestCache_EvictsLeastRecentlyUsedBySize(t *testing.T) {
	// Cap holds two 4-byte entries but not three.
	c, err := mediacache.New(t.TempDir(), 8)
	require.NoError(t, err)

	c.Put("a", []byte("aaaa"))
	time.Sleep(20 * time.Millisecond)
	c.Put("b", []byte("bbbb"))
	time.Sleep(20 * time.Millisecond)
	// Touch "a" so "b" becomes least-recently-used.
	_, _ = c.Get("a")
	time.Sleep(20 * time.Millisecond)
	c.Put("c", []byte("cccc")) // exceeds cap -> evict LRU ("b")

	assert.Equal(t, 2, c.Len())
	_, okB := c.Get("b")
	assert.False(t, okB, "least-recently-used entry should be evicted")
	_, okA := c.Get("a")
	_, okC := c.Get("c")
	assert.True(t, okA)
	assert.True(t, okC)
}

func TestCache_RecencyAndSizeSurviveReopen(t *testing.T) {
	dir := t.TempDir()

	c, err := mediacache.New(dir, 1<<20)
	require.NoError(t, err)
	c.Put("a", []byte("aaaa"))
	c.Put("b", []byte("bbbb"))

	// Reopen: existing files are re-indexed.
	c2, err := mediacache.New(dir, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, 2, c2.Len())
	got, ok := c2.Get("a")
	require.True(t, ok)
	assert.Equal(t, []byte("aaaa"), got)
}

func TestPhotoKey(t *testing.T) {
	assert.Equal(t, "photo_42_m", mediacache.PhotoKey(42, "m"))
	assert.Equal(t, "photo_42", mediacache.PhotoKey(42, ""))
}
