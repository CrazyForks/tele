package keys_test

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/stretchr/testify/assert"
)

func TestDescribe_DefaultFallback(t *testing.T) {
	// "up" has no per-context override; it resolves via the shared default.
	lbl, ok := keys.Describe(keys.ContextChatList, keys.ActionUp)
	assert.True(t, ok)
	assert.Equal(t, "up", lbl.Short)
}

func TestDescribe_ContextOverride(t *testing.T) {
	// down means "move" in a list but "scroll" in the chat pane.
	list, _ := keys.Describe(keys.ContextChatList, keys.ActionDown)
	chat, _ := keys.Describe(keys.ContextChat, keys.ActionDown)
	assert.Equal(t, "move", list.Short)
	assert.Equal(t, "scroll", chat.Short)
}

func TestDescribe_LongDefaultsToShort(t *testing.T) {
	lbl, _ := keys.Describe(keys.ContextChat, keys.ActionReply)
	assert.Equal(t, "reply", lbl.Short)
	assert.Equal(t, "reply", lbl.Long) // Long empty in table -> mirrors Short
}

func TestDescribe_Unknown(t *testing.T) {
	_, ok := keys.Describe(keys.ContextChat, keys.Action("nonexistent"))
	assert.False(t, ok)
}

// Drift guard: every action bound in DefaultKeyMap has a non-empty label in
// its context. A new binding without a label fails here.
func TestDescribe_EveryBoundActionHasLabel(t *testing.T) {
	for ctx, binds := range keys.DefaultKeyMap() {
		for key, action := range binds {
			lbl, ok := keys.Describe(ctx, action)
			assert.Truef(t, ok, "no label for action %q (context %q, key %q)", action, ctx, key)
			assert.NotEmptyf(t, lbl.Short, "empty Short for action %q in context %q", action, ctx)
		}
	}
}
