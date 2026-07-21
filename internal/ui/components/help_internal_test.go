package components

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/stretchr/testify/assert"
)

func TestHelpSections_ChatHasReply(t *testing.T) {
	secs := helpSections(keys.DefaultKeyMap())

	var chat *helpSection
	for i := range secs {
		if secs[i].Title == "Chat" {
			chat = &secs[i]
		}
	}
	assert.NotNil(t, chat, "Chat section present")

	var found bool
	for _, r := range chat.Rows {
		if r.Key == "r" && r.Desc == "reply" {
			found = true
		}
	}
	assert.True(t, found, "Chat section lists 'r  reply'")
}

func TestHelpSections_OrderAndGlobalFirst(t *testing.T) {
	secs := helpSections(keys.DefaultKeyMap())
	assert.NotEmpty(t, secs)
	assert.Equal(t, "Global", secs[0].Title)
	// Message menu folds delete-submenu in; no standalone submenu section.
	for _, s := range secs {
		assert.NotEqual(t, "delete_submenu", s.Title)
		assert.NotEqual(t, "folder_submenu", s.Title)
	}
}
