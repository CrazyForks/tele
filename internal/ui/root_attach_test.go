package ui_test

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// writeTempBytes writes bytes to a real file so MIME detection and os.Stat work.
func writeTempBytes(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// pressCancelUpload drives ActionCancelUpload (x in the chat context). Staging
// leaves the model in insert mode, so esc comes first: otherwise x is typed into
// the caption. In normal mode esc means something else (it moves focus off the
// chat pane), so it is pressed only when there is insert mode to leave.
func pressCancelUpload(t *testing.T, m ui.RootModel) ui.RootModel {
	t.Helper()
	if m.VimMode() == keys.ModeInsert {
		nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
		m = nm.(ui.RootModel)
	}
	nm, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	return nm.(ui.RootModel)
}

// pressToggleSendAs drives ActionToggleSendAs (ctrl+t in the composer context).
func pressToggleSendAs(t *testing.T, m ui.RootModel) ui.RootModel {
	t.Helper()
	nm, _ := m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	return nm.(ui.RootModel)
}

func TestStaging_SecondFileAppendsToQueue(t *testing.T) {
	m, _ := newRootOnChat(t)
	first := writeTempBytes(t, "a.png", pngBytes)
	second := writeTempBytes(t, "b.png", pngBytes)

	nm, _ := m.Update(screens.FileSelectedMsg{Path: first})
	m = nm.(ui.RootModel)
	nm, _ = m.Update(screens.FileSelectedMsg{Path: second})
	m = nm.(ui.RootModel)

	assert.Equal(t, 2, m.PendingAttachmentCount())
	assert.True(t, m.Chat().HasAttachment())
}

func TestStaging_CancelRemovesLastOnly(t *testing.T) {
	m, _ := newRootOnChat(t)
	for _, n := range []string{"a.png", "b.png"} {
		nm, _ := m.Update(screens.FileSelectedMsg{Path: writeTempBytes(t, n, pngBytes)})
		m = nm.(ui.RootModel)
	}

	m = pressCancelUpload(t, m)
	assert.Equal(t, 1, m.PendingAttachmentCount())

	m = pressCancelUpload(t, m)
	assert.Equal(t, 0, m.PendingAttachmentCount())
	assert.False(t, m.Chat().HasAttachment())
}

func TestStaging_ToggleSendAsAppliesToEveryItem(t *testing.T) {
	m, _ := newRootOnChat(t)
	for _, n := range []string{"a.png", "b.png"} {
		nm, _ := m.Update(screens.FileSelectedMsg{Path: writeTempBytes(t, n, pngBytes)})
		m = nm.(ui.RootModel)
	}

	m = pressToggleSendAs(t, m)

	kinds := m.PendingAttachmentSendAsAll()
	require.Len(t, kinds, 2)
	for i, k := range kinds {
		assert.Equal(t, domain.MediaFile, k, "item %d must follow the album-wide toggle", i)
	}
}

func TestStaging_QueueCapRejectsExtraFile(t *testing.T) {
	m, _ := newRootOnChat(t)
	path := writeTempBytes(t, "a.png", pngBytes)
	for i := 0; i < 41; i++ {
		nm, _ := m.Update(screens.FileSelectedMsg{Path: path})
		m = nm.(ui.RootModel)
	}
	assert.Equal(t, 40, m.PendingAttachmentCount(), "the queue is capped at maxStagedAttachments")
}
