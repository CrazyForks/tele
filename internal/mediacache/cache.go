// Package mediacache is a concurrency-safe, size-bounded on-disk cache of media
// files keyed by a filename-safe string. Recency is the file mtime, so the LRU
// order and total-size bound survive process restarts. Nothing here knows what
// a photo or a document is: the owner builds the keys. A miss (or any I/O
// error) transparently falls back to the normal download path. See issues #174
// and #196.
package mediacache

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type entry struct {
	size  int64
	mtime time.Time
}

// Cache is a fixed-total-size LRU over files in a directory. Safe for concurrent
// use: fetches populate it from multiple goroutines.
type Cache struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	index    map[string]entry
	total    int64
}

// New opens (creating if needed) a cache under dir bounded to maxBytes total.
// Existing files are indexed so recency and the size bound carry across restarts.
func New(dir string, maxBytes int64) (*Cache, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	c := &Cache{dir: dir, maxBytes: maxBytes, index: make(map[string]entry)}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		c.index[e.Name()] = entry{size: info.Size(), mtime: info.ModTime()}
		c.total += info.Size()
	}
	c.mu.Lock()
	c.evictLocked("") // an earlier run may have used a larger bound
	c.mu.Unlock()
	return c, nil
}

func (c *Cache) path(key string) string { return filepath.Join(c.dir, key) }

// Path returns the file holding key and marks it most-recently-used. A key the
// cache does not hold, or one whose file has disappeared underneath it, is a
// miss.
//
// The returned file may in principle be evicted before the caller opens it. A
// hit moves the file's mtime, making it the most recently used and therefore
// the last candidate for eviction, so a reader that opens the path promptly
// wins the race in practice. A reader that loses it sees ENOENT and must treat
// that as a miss; do not add a retry here.
func (c *Cache) Path(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.index[key]; !ok {
		return "", false
	}
	p := c.path(key)
	if _, err := os.Stat(p); err != nil {
		c.removeLocked(key)
		return "", false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	e := c.index[key]
	e.mtime = now
	c.index[key] = e
	return p, true
}

// Put stores under key whatever fill writes, and returns the path to the stored
// file. fill receives an empty private temp file and may write, rewind and
// rewrite it: a download retried after an expired file reference does exactly
// that, and only what is in the file when fill returns is kept. A fill that
// returns an error leaves the cache untouched and no temp file behind.
//
// Eviction never removes the entry just written, so the returned path is live
// when Put returns. An entry larger than the whole bound is therefore kept
// until the next Put evicts it; the alternative would be handing back a path to
// a file that was never stored.
func (c *Cache) Put(key string, fill func(*os.File) error) (string, error) {
	// The temp file must be unique per call, not derived from the key. Two
	// concurrent Puts of one key sharing a temp file corrupt each other: the
	// second open truncates what the first is still writing, and once the first
	// renames it into place the second keeps writing through its handle into the
	// published entry. On Windows it also fails outright, because Go opens files
	// without FILE_SHARE_DELETE, so neither rename nor remove works while the
	// other goroutine holds the file open.
	f, err := os.CreateTemp(c.dir, key+".*.tmp")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	if err := fill(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	final := c.path(key)
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.index[key]; ok {
		c.total -= old.size
	}
	c.index[key] = entry{size: size, mtime: time.Now()}
	c.total += size
	c.evictLocked(key)
	return final, nil
}

// Remove deletes key from the cache.
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.index)
}

// removeLocked drops key from the index only once its file is actually gone.
// Deleting an open file fails on Windows; dropping the entry anyway would leave
// the file on disk forever while total no longer counted it, so the bound would
// silently drift.
func (c *Cache) removeLocked(key string) {
	e, ok := c.index[key]
	if !ok {
		return
	}
	if err := os.Remove(c.path(key)); err != nil && !os.IsNotExist(err) {
		return
	}
	c.total -= e.size
	delete(c.index, key)
}

// evictLocked deletes least-recently-used entries until total <= maxBytes,
// never touching keep (the entry Put just wrote).
func (c *Cache) evictLocked(keep string) {
	if c.total <= c.maxBytes {
		return
	}
	keys := make([]string, 0, len(c.index))
	for k := range c.index {
		if k == keep {
			continue
		}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return c.index[keys[i]].mtime.Before(c.index[keys[j]].mtime)
	})
	for _, k := range keys {
		if c.total <= c.maxBytes {
			break
		}
		c.removeLocked(k)
	}
}
