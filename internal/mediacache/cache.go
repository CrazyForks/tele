// Package mediacache is a concurrency-safe, size-bounded on-disk cache of image
// bytes keyed by a filename-safe string. Recency is the file mtime, so the LRU
// order and total-size bound survive process restarts. A miss (or any I/O error)
// transparently falls back to the normal download path. See issue #174.
package mediacache

import (
	"fmt"
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
// use: download commands populate it from multiple goroutines.
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
	c.evictLocked() // an earlier run may have used a larger bound
	c.mu.Unlock()
	return c, nil
}

func (c *Cache) path(key string) string { return filepath.Join(c.dir, key) }

// Get returns the bytes for key and marks it most-recently-used. Missing or
// unreadable entries return (nil, false).
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.index[key]; !ok {
		return nil, false
	}
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		c.removeLocked(key)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(c.path(key), now, now)
	e := c.index[key]
	e.mtime = now
	c.index[key] = e
	return data, true
}

// Put stores data under key (overwriting), marks it most-recently-used, and
// evicts least-recently-used entries until the total size is within the bound.
// Errors are swallowed: a failed write just means a future miss.
func (c *Cache) Put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tmp := c.path(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, c.path(key)); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if old, ok := c.index[key]; ok {
		c.total -= old.size
	}
	c.index[key] = entry{size: int64(len(data)), mtime: time.Now()}
	c.total += int64(len(data))
	c.evictLocked()
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

func (c *Cache) removeLocked(key string) {
	if e, ok := c.index[key]; ok {
		c.total -= e.size
		delete(c.index, key)
		_ = os.Remove(c.path(key))
	}
}

// evictLocked deletes least-recently-used entries until total <= maxBytes.
func (c *Cache) evictLocked() {
	if c.total <= c.maxBytes {
		return
	}
	keys := make([]string, 0, len(c.index))
	for k := range c.index {
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

// PhotoKey is the cache key for an inline photo of the given id and thumb size.
func PhotoKey(id int64, thumbSize string) string {
	if thumbSize == "" {
		return fmt.Sprintf("photo_%d", id)
	}
	return fmt.Sprintf("photo_%d_%s", id, thumbSize)
}
