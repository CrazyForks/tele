// Package imagecache provides a count-bounded LRU cache of decoded images,
// keyed by Telegram photo/document id. Decoded images are pixel-bounded
// (MaxLongSidePx), so a count cap bounds total memory predictably. Replacing
// the previous unbounded maps keeps long browsing sessions from growing memory
// monotonically; a miss transparently re-triggers the existing download path.
package imagecache

import (
	"image"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Cache is a fixed-capacity LRU over decoded images. It is not safe for
// concurrent use; callers drive it from the single bubbletea update/view
// goroutine.
type Cache struct {
	lru *lru.Cache[int64, image.Image]
}

// New returns a cache holding at most size entries. size must be positive;
// lru.New only errors on a non-positive size, which is a programming error here.
func New(size int) *Cache {
	c, err := lru.New[int64, image.Image](size)
	if err != nil {
		panic(err)
	}
	return &Cache{lru: c}
}

// Get returns the image for id and marks it most-recently-used.
func (c *Cache) Get(id int64) (image.Image, bool) { return c.lru.Get(id) }

// Peek returns the image for id without affecting recency.
func (c *Cache) Peek(id int64) (image.Image, bool) { return c.lru.Peek(id) }

// Contains reports whether id is cached without affecting recency.
func (c *Cache) Contains(id int64) bool { return c.lru.Contains(id) }

// Add inserts or updates id, marking it most-recently-used and evicting the
// least-recently-used entry if the cache is full.
func (c *Cache) Add(id int64, img image.Image) { c.lru.Add(id, img) }

// Remove deletes id from the cache if present.
func (c *Cache) Remove(id int64) { c.lru.Remove(id) }

// Len returns the number of cached entries.
func (c *Cache) Len() int { return c.lru.Len() }
