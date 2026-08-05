package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// S must follow Apply: styles are rebuilt from the theme in the applied slot.
func TestS_FollowsAppliedSlot(t *testing.T) {
	theme.Apply(true)
	dark := theme.S().HelpDesc.GetForeground()

	theme.Apply(false)
	light := theme.S().HelpDesc.GetForeground()

	assert.NotEqual(t, dark, light, "help body text must differ between slots")
}

// Styles carry their non-color attributes, not just the color.
func TestS_KeepsNonColorAttributes(t *testing.T) {
	theme.Apply(true)
	s := theme.S()
	assert.True(t, s.NameIncoming.GetBold(), "incoming sender name is bold")
	assert.True(t, s.NameEditing.GetBold(), "edited-name marker is bold")
	assert.True(t, s.MentionStatus.GetItalic(), "mention popup status line is italic")
}

// The styles pointer and the theme pointer are swapped together, so a render
// can never mix a new theme with old styles.
func TestS_MatchesT(t *testing.T) {
	theme.Apply(false)
	require.Equal(t, "tele-light", theme.T().Name)
	assert.Equal(t, theme.T().TextDim, theme.S().Timestamp.GetForeground())
}
