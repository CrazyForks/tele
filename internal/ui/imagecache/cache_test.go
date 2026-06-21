package imagecache

import (
	"image"
	"testing"
)

// img returns a distinct 1x1 image so pointers differ per id.
func img() image.Image { return image.NewRGBA(image.Rect(0, 0, 1, 1)) }

func TestAddGetAndMiss(t *testing.T) {
	c := New(2)
	a := img()
	c.Add(1, a)
	got, ok := c.Get(1)
	if !ok || got != a {
		t.Fatalf("Get(1) = %v, %v; want stored image, true", got, ok)
	}
	if _, ok := c.Get(99); ok {
		t.Fatalf("Get(99) ok = true; want false for a miss")
	}
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := New(2)
	c.Add(1, img())
	c.Add(2, img())
	c.Add(3, img()) // evicts 1 (LRU)
	if c.Contains(1) {
		t.Fatalf("id 1 should have been evicted")
	}
	if !c.Contains(2) || !c.Contains(3) {
		t.Fatalf("ids 2 and 3 should be present")
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d; want 2", c.Len())
	}
}

func TestGetBumpsRecency(t *testing.T) {
	c := New(2)
	c.Add(1, img())
	c.Add(2, img())
	c.Get(1)        // 1 is now most-recently-used
	c.Add(3, img()) // evicts 2, not 1
	if !c.Contains(1) {
		t.Fatalf("id 1 was bumped by Get and must survive")
	}
	if c.Contains(2) {
		t.Fatalf("id 2 was least-recently-used and should be evicted")
	}
}

func TestPeekAndContainsDoNotBump(t *testing.T) {
	c := New(2)
	c.Add(1, img())
	c.Add(2, img())
	c.Peek(1)     // must NOT bump recency
	c.Contains(1) // must NOT bump recency
	c.Add(3, img())
	if c.Contains(1) {
		t.Fatalf("Peek/Contains must not bump recency; id 1 should be evicted")
	}
}

func TestRemove(t *testing.T) {
	c := New(2)
	c.Add(1, img())
	c.Remove(1)
	if c.Contains(1) {
		t.Fatalf("id 1 should be gone after Remove")
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d; want 0", c.Len())
	}
}
