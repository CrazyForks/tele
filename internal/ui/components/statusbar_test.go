package components_test

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/stretchr/testify/assert"
)

// strip removes ANSI styling so assertions can match the visible text even
// when a hint's key letter is colored mid-word.
func strip(s string) string { return xansi.Strip(s) }

func TestStatusBar_NormalMode(t *testing.T) {
	sb := components.NewStatusBar(80)
	sb.SetMode(keys.ModeNormal)
	assert.Contains(t, strip(sb.View()), "NORMAL")
}

func TestStatusBar_InsertMode(t *testing.T) {
	sb := components.NewStatusBar(80)
	sb.SetMode(keys.ModeInsert)
	assert.Contains(t, strip(sb.View()), "INSERT")
}

func TestStatusBar_StatusText(t *testing.T) {
	sb := components.NewStatusBar(80)
	sb.SetStatus("Loading...")
	assert.Contains(t, strip(sb.View()), "Loading...")
}

func TestStatusBar_ChatHintsIncludeAttach(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(160)
	sb.SetKeyMap(km)
	sb.SetActivePane("chat")
	sb.SetMode(keys.ModeNormal)
	assert.Contains(t, strip(sb.View()), "f attach")
}

func TestStatusBar_PickerHints(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(160)
	sb.SetKeyMap(km)
	sb.SetActivePane("chat")
	sb.SetPickerOpen(true)
	view := strip(sb.View())
	assert.Contains(t, view, "type filter")
	assert.Contains(t, view, "open/select ↵")
}

func TestStatusBar_StagedAttachmentHints(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(160)
	sb.SetKeyMap(km)
	sb.SetActivePane("chat")
	sb.SetAttachStaged(true)

	sb.SetMode(keys.ModeInsert)
	insertView := strip(sb.View())
	assert.Contains(t, insertView, "send ↵")
	assert.Contains(t, insertView, "ctrl+t photo/file")

	sb.SetMode(keys.ModeNormal)
	assert.Contains(t, strip(sb.View()), "x drop file")
}

func TestStatusBar_ChatlistHints(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(120)
	sb.SetKeyMap(km)
	sb.SetActivePane("chatlist")
	view := strip(sb.View())
	assert.Contains(t, view, "j/k move")
	assert.Contains(t, view, "open ↵")
	assert.Contains(t, view, "/ search")
	assert.Contains(t, view, "quit")
	assert.NotContains(t, view, "->")
}

func TestStatusBar_ChatNormalHints(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(120)
	sb.SetKeyMap(km)
	sb.SetActivePane("chat")
	sb.SetMode(keys.ModeNormal)
	view := strip(sb.View())
	assert.Contains(t, view, "j/k scroll")
	assert.Contains(t, view, "ctrl+j/k select")
	assert.NotContains(t, view, "menu")
	assert.Contains(t, view, "a write")
	assert.Contains(t, view, "quit")
	assert.NotContains(t, view, "->")
}

func TestStatusBar_ChatInsertHints(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(120)
	sb.SetKeyMap(km)
	sb.SetActivePane("chat")
	sb.SetMode(keys.ModeInsert)
	view := strip(sb.View())
	assert.Contains(t, view, "send ↵")
	assert.Contains(t, view, "esc normal")
}

func TestStatusBar_NoKeyMap_NoHints(t *testing.T) {
	sb := components.NewStatusBar(80)
	sb.SetActivePane("chatlist")
	view := strip(sb.View())
	assert.NotContains(t, view, "->")
}

func TestStatusBar_SetError_ShownInView(t *testing.T) {
	sb := components.NewStatusBar(80)
	sb.SetError("download failed", components.SeverityError)
	assert.Contains(t, strip(sb.View()), "download failed")
}

func TestStatusBar_ClearError_StaleSerialIsNoop(t *testing.T) {
	sb := components.NewStatusBar(80)
	serial := sb.SetError("first", components.SeverityError)
	sb.SetError("second", components.SeverityWarning) // bumps serial
	sb.ClearError(serial)                             // stale → ignored
	assert.Contains(t, strip(sb.View()), "second")
}

func TestStatusBar_ClearError_CurrentSerialClears(t *testing.T) {
	sb := components.NewStatusBar(80)
	serial := sb.SetError("boom", components.SeverityError)
	sb.ClearError(serial)
	assert.NotContains(t, strip(sb.View()), "boom")
}

func TestStatusBar_SeparatesSegments(t *testing.T) {
	sb := components.NewStatusBar(120)
	sb.SetKeyMap(keys.DefaultKeyMap())
	sb.SetActivePane("chatlist")
	sb.SetError("network down", components.SeverityError)
	view := strip(sb.View())
	assert.Contains(t, view, "network down")
	assert.Contains(t, view, "│") // segment separator present
}

func TestStatusBar_ErrorReplacesStatus(t *testing.T) {
	sb := components.NewStatusBar(120)
	sb.SetStatus("12 chats")
	sb.SetError("boom", components.SeverityError)
	view := strip(sb.View())
	assert.Contains(t, view, "boom")
	assert.NotContains(t, view, "12 chats")
}

func TestStatusBar_HintsAppendedAfterStatus(t *testing.T) {
	km := keys.DefaultKeyMap()
	sb := components.NewStatusBar(120)
	sb.SetKeyMap(km)
	sb.SetActivePane("chatlist")
	sb.SetStatus("12 chats")
	view := strip(sb.View())
	statusIdx := strings.Index(view, "12 chats")
	hintIdx := strings.Index(view, "j/k move")
	assert.Greater(t, hintIdx, statusIdx, "hints should appear after status text")
}
