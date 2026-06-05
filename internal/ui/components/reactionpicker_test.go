package components_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pressLeft() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: tea.KeyLeft} }
func pressRight() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyRight} }

func TestReactionPicker_Enter_EmitsConfirmed(t *testing.T) {
	p := components.NewReactionPicker("")
	newP, cmd := p.Update(pressEnter())
	assert.Nil(t, newP)
	require.NotNil(t, cmd)
	msg, ok := cmd().(components.ReactConfirmedMsg)
	require.True(t, ok)
	assert.Equal(t, "🤝", msg.Emoji) // top-left of grid
}

func TestReactionPicker_Esc_EmitsClose(t *testing.T) {
	p := components.NewReactionPicker("")
	newP, cmd := p.Update(pressEsc())
	assert.Nil(t, newP)
	require.NotNil(t, cmd)
	_, ok := cmd().(components.CloseReactionPickerMsg)
	assert.True(t, ok)
}

func TestReactionPicker_RightMoves(t *testing.T) {
	p := components.NewReactionPicker("")
	p, _ = p.Update(pressRight())
	require.NotNil(t, p)
	newP, cmd := p.Update(pressEnter())
	assert.Nil(t, newP)
	require.NotNil(t, cmd)
	msg := cmd().(components.ReactConfirmedMsg)
	assert.Equal(t, "🙏", msg.Emoji) // one step right from top-left
}

func TestReactionPicker_DownMoves(t *testing.T) {
	p := components.NewReactionPicker("")
	p, _ = p.Update(pressDown())
	require.NotNil(t, p)
	newP, cmd := p.Update(pressEnter())
	assert.Nil(t, newP)
	require.NotNil(t, cmd)
	msg := cmd().(components.ReactConfirmedMsg)
	assert.Equal(t, "💯", msg.Emoji) // one step down from top-left
}

func TestReactionPicker_LeftAtEdgeStays(t *testing.T) {
	p := components.NewReactionPicker("")
	p, _ = p.Update(pressLeft())
	require.NotNil(t, p)
	newP, cmd := p.Update(pressEnter())
	assert.Nil(t, newP)
	msg := cmd().(components.ReactConfirmedMsg)
	assert.Equal(t, "🤝", msg.Emoji) // stays at top-left
}

func TestReactionPicker_ViewContainsEmoji(t *testing.T) {
	p := components.NewReactionPicker("")
	v := p.View()
	assert.Contains(t, v, "🤝")
	assert.Contains(t, v, "🤬")
}
