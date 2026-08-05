package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// S must follow Apply: styles are rebuilt from the flavour that was applied.
func TestS_FollowsAppliedFlavour(t *testing.T) {
	theme.Apply(theme.Default, true)
	dark := theme.S().HelpDesc.GetForeground()

	theme.Apply(theme.Default, false)
	light := theme.S().HelpDesc.GetForeground()

	assert.NotEqual(t, dark, light, "help body text must differ between flavours")
}

// Styles carry their non-color attributes, not just the color.
func TestS_KeepsNonColorAttributes(t *testing.T) {
	theme.Apply(theme.Default, true)
	s := theme.S()
	assert.True(t, s.NameIncoming.GetBold(), "incoming sender name is bold")
	assert.True(t, s.NameEditing.GetBold(), "edited-name marker is bold")
	assert.True(t, s.MentionStatus.GetItalic(), "mention popup status line is italic")
}

// The styles pointer and the theme pointer are swapped together, so a render
// can never mix a new theme with old styles.
func TestS_MatchesT(t *testing.T) {
	theme.Apply(theme.Default, false)
	require.False(t, theme.T().Dark)
	assert.Equal(t, theme.T().TextDim, theme.S().Timestamp.GetForeground())
}
