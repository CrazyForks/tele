package components_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyMsg is a helper that builds a tea.KeyPressMsg from a rune.
func keyMsg(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func pressJ() tea.KeyPressMsg { return keyMsg('j') }
func pressK() tea.KeyPressMsg { return keyMsg('k') }

func pressDown() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: tea.KeyDown} }
func pressUp() tea.KeyPressMsg    { return tea.KeyPressMsg{Code: tea.KeyUp} }
func pressEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func pressEsc() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEsc} }
func pressSpace() tea.KeyPressMsg { return keyMsg(' ') }

func TestNewContextMenu_IncomingItems(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	view := cm.View()
	assert.Contains(t, view, "Reply")
	assert.Contains(t, view, "React")
	assert.Contains(t, view, "Delete")
	assert.NotContains(t, view, "Edit")
}

func TestNewContextMenu_OutgoingItems(t *testing.T) {
	cm := components.NewContextMenu(1, true)
	view := cm.View()
	assert.Contains(t, view, "Reply")
	assert.Contains(t, view, "React")
	assert.Contains(t, view, "Edit")
	assert.Contains(t, view, "Delete")
}

func TestNewContextMenu_CursorStartsOnReply(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	assert.Contains(t, cm.View(), "▸ Reply")
}

func TestContextMenu_J_MovesCursorDown(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	cm, _ = cm.Update(pressJ())
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ React")
	assert.NotContains(t, cm.View(), "▸ Reply")
}

func TestContextMenu_DownArrow_MovesCursorDown(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	cm, _ = cm.Update(pressDown())
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ React")
}

func TestContextMenu_K_MovesCursorUp(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	// Move down to React, then back up to Reply
	cm, _ = cm.Update(pressJ())
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressK())
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ Reply")
}

func TestContextMenu_UpArrow_MovesCursorUp(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	cm, _ = cm.Update(pressDown())
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressUp())
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ Reply")
}

func TestContextMenu_WrapAround_K_FromFirst_GoesToLast(t *testing.T) {
	// incoming: Reply(0), React(1), Delete(2)
	// pressing k from Reply should wrap to Delete
	cm := components.NewContextMenu(1, false)
	cm, _ = cm.Update(pressK()) // wrap from Reply to Delete
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ Delete")
}

func TestContextMenu_EscFromMain_Closes(t *testing.T) {
	cm := components.NewContextMenu(42, false)
	newCM, cmd := cm.Update(pressEsc())
	assert.Nil(t, newCM, "menu should close")
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_Space_Closes(t *testing.T) {
	cm := components.NewContextMenu(42, false)
	newCM, cmd := cm.Update(pressSpace())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_ReplyStub_Closes(t *testing.T) {
	cm := components.NewContextMenu(42, false)
	// cursor is on Reply (index 0)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_ReactStub_Closes(t *testing.T) {
	cm := components.NewContextMenu(42, false)
	cm, _ = cm.Update(pressJ()) // React
	require.NotNil(t, cm)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_EditStub_Closes(t *testing.T) {
	cm := components.NewContextMenu(42, true)
	cm, _ = cm.Update(pressJ()) // React
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Edit
	require.NotNil(t, cm)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_DeleteIncoming_EmitsDeleteRequest(t *testing.T) {
	cm := components.NewContextMenu(42, false)
	// incoming: Reply(0), React(1), Delete(2)
	cm, _ = cm.Update(pressJ()) // React
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Delete
	require.NotNil(t, cm)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	msg := cmd()
	req, ok := msg.(components.DeleteMsgRequest)
	require.True(t, ok)
	assert.Equal(t, 42, req.MsgID)
	assert.False(t, req.Revoke)
}

func TestContextMenu_DeleteOutgoing_ShowsSubPrompt(t *testing.T) {
	cm := components.NewContextMenu(42, true)
	// outgoing: Reply(0), React(1), Edit(2), Delete(3)
	cm, _ = cm.Update(pressJ()) // React
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Edit
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Delete
	require.NotNil(t, cm)
	newCM, cmd := cm.Update(pressEnter()) // enter on Delete
	require.NotNil(t, newCM, "menu stays open (sub-prompt)")
	assert.Nil(t, cmd)
	view := newCM.View()
	assert.Contains(t, view, "For everyone")
	assert.Contains(t, view, "For me")
	assert.Contains(t, view, "Cancel")
	assert.NotContains(t, view, "Reply")
}

func TestContextMenu_DeleteSub_ForMe_EmitsDelete(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	// cursor on "For me" (index 0)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	req, ok := cmd().(components.DeleteMsgRequest)
	require.True(t, ok)
	assert.Equal(t, 99, req.MsgID)
	assert.False(t, req.Revoke)
}

func TestContextMenu_DeleteSub_ForEveryone_EmitsDeleteRevoke(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	cm, _ = cm.Update(pressJ()) // For everyone
	require.NotNil(t, cm)
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	req, ok := cmd().(components.DeleteMsgRequest)
	require.True(t, ok)
	assert.True(t, req.Revoke)
}

func TestContextMenu_DeleteSub_Cancel_Closes(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	// Navigate to Cancel (index 3), skipping separator (index 2)
	cm, _ = cm.Update(pressJ()) // For everyone
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // should skip separator, land on Cancel
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ Cancel")
	newCM, cmd := cm.Update(pressEnter())
	assert.Nil(t, newCM)
	require.NotNil(t, cmd)
	assert.IsType(t, components.CloseContextMenuMsg{}, cmd())
}

func TestContextMenu_DeleteSub_SeparatorSkipped_Down(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	cm, _ = cm.Update(pressJ()) // For everyone (index 1)
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // should skip sep(2), land on Cancel(3)
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ Cancel")
}

func TestContextMenu_DeleteSub_SeparatorSkipped_Up(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	cm, _ = cm.Update(pressJ()) // For everyone
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Cancel (skipping sep)
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressK()) // should skip sep going up, land on For everyone
	require.NotNil(t, cm)
	assert.Contains(t, cm.View(), "▸ For everyone")
}

func TestContextMenu_EscFromSubPrompt_ReturnsToMain(t *testing.T) {
	cm := navigateToDeleteSubPrompt(t)
	newCM, cmd := cm.Update(pressEsc())
	require.NotNil(t, newCM, "should NOT close, should return to main")
	assert.Nil(t, cmd)
	view := newCM.View()
	assert.Contains(t, view, "Reply")
	assert.Contains(t, view, "Delete")
	assert.NotContains(t, view, "For me")
}

func TestContextMenu_View_ReturnsNonEmpty(t *testing.T) {
	cm := components.NewContextMenu(1, false)
	assert.NotEmpty(t, cm.View())
}

// navigateToDeleteSubPrompt creates an outgoing ContextMenu with cursor on the
// Delete sub-prompt for msgID=99.
func navigateToDeleteSubPrompt(t *testing.T) *components.ContextMenu {
	t.Helper()
	cm := components.NewContextMenu(99, true)
	// outgoing: Reply(0) React(1) Edit(2) Delete(3)
	cm, _ = cm.Update(pressJ()) // React
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Edit
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressJ()) // Delete
	require.NotNil(t, cm)
	cm, _ = cm.Update(pressEnter()) // enter sub-prompt
	require.NotNil(t, cm)
	return cm
}
