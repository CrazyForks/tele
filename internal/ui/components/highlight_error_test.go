package components

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

func TestErrorAccent_DiffersByFlavour(t *testing.T) {
	theme.Apply(theme.Default, true)
	dark := theme.T().HighlightError
	theme.Apply(theme.Default, false)
	assert.NotEqual(t, dark, theme.T().HighlightError, "the error accent must adapt to the background")
}

func TestErrorAccent_MatchesToastErrorRed(t *testing.T) {
	// The rollback highlight and the failure toast must read as one color.
	theme.Apply(theme.Default, true)
	assert.Equal(t, theme.T().StatusError, theme.T().HighlightError)
}

func TestHighlightKind_InfoIsZero(t *testing.T) {
	assert.Equal(t, HighlightKind(0), HighlightInfo)
	assert.NotEqual(t, HighlightInfo, HighlightError)
}
