package components_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

func TestRenderNoticeBox_ShowsTitleBodyAndCountdown(t *testing.T) {
	out := components.RenderNoticeBox("State moved", "Your session now lives elsewhere.", 7, 60)

	assert.Contains(t, out, "State moved")
	assert.Contains(t, out, "Your session now lives elsewhere.")
	assert.Contains(t, out, "7")
}

func TestRenderNoticeBox_PromptsWhenCountdownDone(t *testing.T) {
	out := components.RenderNoticeBox("Title", "Body", 0, 60)

	assert.Contains(t, strings.ToLower(out), "any key")
}

func TestRenderNoticeBox_WrapsWithinMaxWidth(t *testing.T) {
	long := strings.Repeat("word ", 60)
	out := components.RenderNoticeBox("Title", long, 0, 40)

	// Measure display width, not rune count: the footer carries ANSI styling.
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40, "line exceeds the box: %q", line)
	}
}
