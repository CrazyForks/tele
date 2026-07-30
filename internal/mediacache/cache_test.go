package mediacache_test

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/mediacache"
)

// write is the common fill: it dumps a fixed payload into the cache file.
func write(payload string) func(*os.File) error {
	return func(f *os.File) error {
		_, err := f.Write([]byte(payload))
		return err
	}
}

func TestCache_PutReturnsAReadablePath(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	path, err := c.Put("a", write("hello"))

	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestCache_PathReturnsTheStoredFile(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)
	stored, err := c.Put("a", write("hello"))
	require.NoError(t, err)

	path, ok := c.Path("a")

	require.True(t, ok)
	assert.Equal(t, stored, path)
}

func TestCache_PathMissesForAnUnknownKey(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	_, ok := c.Path("absent")

	assert.False(t, ok)
}

// A file removed behind the cache's back is a miss, not a path to nothing.
func TestCache_PathMissesWhenTheFileVanished(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)
	path, err := c.Put("a", write("hello"))
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	_, ok := c.Path("a")

	assert.False(t, ok)
	assert.Equal(t, 0, c.Len(), "a vanished entry must leave the index")
}

func TestCache_EvictsLeastRecentlyUsedBySize(t *testing.T) {
	// The bound holds two 4-byte entries but not three.
	c, err := mediacache.New(t.TempDir(), 8)
	require.NoError(t, err)

	_, err = c.Put("a", write("aaaa"))
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	_, err = c.Put("b", write("bbbb"))
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	// Touch "a" so "b" becomes least-recently-used.
	_, ok := c.Path("a")
	require.True(t, ok)
	time.Sleep(20 * time.Millisecond)

	_, err = c.Put("c", write("cccc"))
	require.NoError(t, err)

	_, aOK := c.Path("a")
	_, bOK := c.Path("b")
	_, cOK := c.Path("c")
	assert.True(t, aOK, "recently touched entry must survive")
	assert.False(t, bOK, "least recently used entry must be evicted")
	assert.True(t, cOK)
}

// The path Put returns must be live when it returns, even for an entry that
// alone exceeds the bound. It is evicted by the next Put instead.
func TestCache_PutNeverEvictsTheEntryItJustWrote(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 4)
	require.NoError(t, err)

	path, err := c.Put("big", write("aaaaaaaaaaaa"))

	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestCache_RecoversTheIndexFromDiskOnOpen(t *testing.T) {
	dir := t.TempDir()
	c, err := mediacache.New(dir, 1<<20)
	require.NoError(t, err)
	_, err = c.Put("a", write("hello"))
	require.NoError(t, err)

	reopened, err := mediacache.New(dir, 1<<20)
	require.NoError(t, err)

	_, ok := reopened.Path("a")
	assert.True(t, ok)
	assert.Equal(t, 1, reopened.Len())
}

func TestCache_LeavesNoTempFileWhenFillFails(t *testing.T) {
	dir := t.TempDir()
	c, err := mediacache.New(dir, 1<<20)
	require.NoError(t, err)

	_, err = c.Put("a", func(*os.File) error { return errors.New("network died") })

	require.Error(t, err)
	_, ok := c.Path("a")
	assert.False(t, ok)
	ents, rerr := os.ReadDir(dir)
	require.NoError(t, rerr)
	assert.Empty(t, ents, "a failed write must leave nothing behind")
}

// fill may rewind and rewrite: a download retried after an expired file
// reference does exactly that. Only the final contents are kept.
func TestCache_KeepsOnlyWhatFillLeftBehindAfterARewrite(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	path, err := c.Put("a", func(f *os.File) error {
		if _, werr := f.Write([]byte("first attempt, longer")); werr != nil {
			return werr
		}
		if _, serr := f.Seek(0, 0); serr != nil {
			return serr
		}
		if terr := f.Truncate(0); terr != nil {
			return terr
		}
		_, werr := f.Write([]byte("second"))
		return werr
	})

	require.NoError(t, err)
	got, rerr := os.ReadFile(path)
	require.NoError(t, rerr)
	assert.Equal(t, "second", string(got))
}

func TestCache_RemoveDropsTheEntry(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)
	_, err = c.Put("a", write("hello"))
	require.NoError(t, err)

	c.Remove("a")

	_, ok := c.Path("a")
	assert.False(t, ok)
	assert.Equal(t, 0, c.Len())
}

// Two Put calls for one key must each get their own temp file. Deriving the
// temp name from the key alone makes them share one: the second open truncates
// what the first is still writing, and whichever renames first moves the shared
// file out from under the other, so a fill that completed fine still reports
// failure. Overlapping fetches of one media file are ordinary, so both calls
// have to succeed and the last one to finish owns the entry.
func TestCache_ConcurrentPutsOfOneKeyDoNotShareATempFile(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	slowOpened := make(chan struct{})
	fastDone := make(chan struct{})

	var fastErr error
	go func() {
		defer close(fastDone)
		<-slowOpened
		_, fastErr = c.Put("a", write("fast"))
	}()

	// The slow fill holds its file open across the whole of the other Put, so
	// the fast one has finished renaming before this one writes a byte.
	slowPath, slowErr := c.Put("a", func(f *os.File) error {
		close(slowOpened)
		<-fastDone
		_, werr := f.Write([]byte("slow"))
		return werr
	})

	require.NoError(t, fastErr)
	require.NoError(t, slowErr)

	got, rerr := os.ReadFile(slowPath)
	require.NoError(t, rerr)
	assert.Equal(t, "slow", string(got))
	assert.Equal(t, 1, c.Len())
}

func TestCache_SurvivesConcurrentPutsAndReads(t *testing.T) {
	c, err := mediacache.New(t.TempDir(), 1<<20)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "k" + strconv.Itoa(i%5)
			_, _ = c.Put(key, write("payload"))
			_, _ = c.Path(key)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 5, c.Len())
}
