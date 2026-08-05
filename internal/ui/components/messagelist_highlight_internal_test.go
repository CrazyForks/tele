package components

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

func TestBubbleBorderFg_NormalWhenNoHighlight(t *testing.T) {
	theme.Apply(theme.Default, true)
	ml := NewMessageList(20, 80)
	in := ml.bubbleBorderFg(domain.Message{ID: 1, IsOut: false})
	assert.Equal(t, theme.T().BorderBubbleIn, in)
	out := ml.bubbleBorderFg(domain.Message{ID: 2, IsOut: true})
	assert.Equal(t, theme.T().BorderBubbleOut, out)
}

func TestBubbleBorderFg_AccentWhenHighlighted_Dark(t *testing.T) {
	theme.Apply(theme.Default, true)
	ml := NewMessageList(20, 80)
	ml.HighlightMessage(1)
	got := ml.bubbleBorderFg(domain.Message{ID: 1, IsOut: false})
	want := FadeAccentColor(theme.T().HighlightAccent, theme.T().BorderBubbleIn, HighlightFadeSteps, HighlightFadeSteps)
	assert.Equal(t, want, got)
}

func TestBubbleBorderFg_AccentWhenHighlighted_Light(t *testing.T) {
	theme.Apply(theme.Default, false)
	ml := NewMessageList(20, 80)
	ml.HighlightMessage(1)
	got := ml.bubbleBorderFg(domain.Message{ID: 1, IsOut: false})
	want := FadeAccentColor(theme.T().HighlightAccent, theme.T().BorderBubbleIn, HighlightFadeSteps, HighlightFadeSteps)
	assert.Equal(t, want, got)
	// The light accent must differ from the dark one.
	assert.NotEqual(t, theme.Default.Dark.HighlightAccent, theme.Default.Light.HighlightAccent)
}

func TestBubbleBorderFg_OtherMessagesUnaffected(t *testing.T) {
	theme.Apply(theme.Default, true)
	ml := NewMessageList(20, 80)
	ml.HighlightMessage(1)
	got := ml.bubbleBorderFg(domain.Message{ID: 99, IsOut: false})
	assert.Equal(t, theme.T().BorderBubbleIn, got)
}

func TestBubbleBorderFg_ErrorAccentWhenHighlighted_Dark(t *testing.T) {
	theme.Apply(theme.Default, true)
	ml := NewMessageList(20, 80)
	ml.HighlightMessageError(1)
	got := ml.bubbleBorderFg(domain.Message{ID: 1, IsOut: false})
	want := FadeAccentColor(theme.T().HighlightError, theme.T().BorderBubbleIn, HighlightFadeSteps, HighlightFadeSteps)
	assert.Equal(t, want, got)
	assert.Equal(t, HighlightError, ml.HighlightKind())
}

func TestBubbleBorderFg_ErrorAccentWhenHighlighted_Light(t *testing.T) {
	theme.Apply(theme.Default, false)
	ml := NewMessageList(20, 80)
	ml.HighlightMessageError(1)
	got := ml.bubbleBorderFg(domain.Message{ID: 1, IsOut: false})
	want := FadeAccentColor(theme.T().HighlightError, theme.T().BorderBubbleIn, HighlightFadeSteps, HighlightFadeSteps)
	assert.Equal(t, want, got)
}

func TestHighlightMessage_SetsInfoKind(t *testing.T) {
	ml := NewMessageList(20, 80)
	ml.HighlightMessage(1)
	assert.Equal(t, HighlightInfo, ml.HighlightKind())
	assert.Equal(t, HighlightInitialStep, ml.HighlightStep())
}

func TestHighlightMessageError_SetsStepAndID(t *testing.T) {
	ml := NewMessageList(20, 80)
	ml.HighlightMessageError(42)
	assert.Equal(t, 42, ml.HighlightedMsgID())
	assert.Equal(t, HighlightInitialStep, ml.HighlightStep())
}
