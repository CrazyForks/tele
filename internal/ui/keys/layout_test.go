package keys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeKey_RussianToLatinSamePhysicalKey(t *testing.T) {
	cases := map[string]string{
		"к":      "r", // physical R
		"й":      "q",
		"о":      "j",
		"л":      "k",
		"у":      "e",
		"ф":      "a",
		"П":      "G", // shifted: physical G
		"ctrl+в": "ctrl+d",
		"ctrl+г": "ctrl+u",
		// Already-Latin and named keys are unchanged.
		"r":     "r",
		"enter": "enter",
		"esc":   "esc",
		"up":    "up",
		"/":     "/",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeKey(in), "normalizeKey(%q)", in)
	}
}

func TestKeyMapResolve_RussianLayoutResolvesLatinBinding(t *testing.T) {
	km := DefaultKeyMap()
	// 'к' is the Russian char on the physical R key, bound to reply in chat.
	assert.Equal(t, ActionReply, km.Resolve(ContextChat, "к"))
	// Latin still works.
	assert.Equal(t, ActionReply, km.Resolve(ContextChat, "r"))
	// Global fallback under Russian layout: 'й' (physical Q) -> quit.
	assert.Equal(t, ActionQuit, km.Resolve(ContextChat, "й"))
}

func TestMatcherResolve_RussianLayoutChord(t *testing.T) {
	m := NewMatcher(DefaultKeyMap())
	// go_top is the chord "g g"; physical G is 'п' on a Russian layout.
	_, r1 := m.Resolve(ContextChat, "п")
	assert.Equal(t, MatchPending, r1)
	a, r2 := m.Resolve(ContextChat, "п")
	assert.Equal(t, MatchAction, r2)
	assert.Equal(t, ActionGoTop, a)
}
