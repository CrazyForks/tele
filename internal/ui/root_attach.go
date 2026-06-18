package ui

import (
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/media"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// pendingAttachment is the staged file awaiting send. kind is the MIME-detected
// default; sendAs is the user's choice (Photo/File toggle). The File branch is #129.
type pendingAttachment struct {
	path   string
	mime   string
	kind   store.MediaKind
	sendAs store.MediaKind
	name   string
	size   int64
}

func (m RootModel) openFilePicker() (RootModel, tea.Cmd) {
	m.filePicker = screens.NewFilePickerModel(m.lastPickerDir, m.width, m.height, m.keyMap)
	m.statusBar.SetPickerOpen(true)
	return m, nil
}

func (m RootModel) handleFileSelected(msg screens.FileSelectedMsg) (RootModel, tea.Cmd) {
	if m.filePicker != nil {
		m.lastPickerDir = m.filePicker.Dir()
	}
	m.filePicker = nil
	m.statusBar.SetPickerOpen(false)

	mime, err := media.DetectMIME(msg.Path)
	if err != nil {
		return m, func() tea.Msg {
			return StatusErrMsg{Text: "cannot read file", Sev: components.SeverityWarning}
		}
	}
	kind := media.DefaultMediaType(mime)
	name, size := fileNameSize(msg.Path)
	m.pendingAttachment = &pendingAttachment{
		path:   msg.Path,
		mime:   mime,
		kind:   kind,
		sendAs: kind,
		name:   name,
		size:   size,
	}
	toggleable := kind == store.MediaPhoto || kind == store.MediaVideo
	m.chat.SetAttachment(name, size, m.pendingAttachment.sendAs, toggleable)
	m.statusBar.SetAttachStaged(true)
	// Enter real insert mode so the caption field is active (the composer focus
	// alone does not flip the root's vim mode, which key routing depends on).
	m.vimState.Mode = keys.ModeInsert
	m.statusBar.SetMode(keys.ModeInsert)
	return m, m.chat.FocusComposer()
}

// toggleSendAs flips the staged attachment between its native kind and File.
// Only image/video are toggleable; the File branch hands off to #129.
func (m RootModel) toggleSendAs() (RootModel, tea.Cmd) {
	if m.pendingAttachment == nil {
		return m, nil
	}
	if m.pendingAttachment.kind != store.MediaPhoto && m.pendingAttachment.kind != store.MediaVideo {
		return m, nil
	}
	if m.pendingAttachment.sendAs == store.MediaFile {
		m.pendingAttachment.sendAs = m.pendingAttachment.kind
	} else {
		m.pendingAttachment.sendAs = store.MediaFile
	}
	m.chat.SetAttachment(m.pendingAttachment.name, m.pendingAttachment.size, m.pendingAttachment.sendAs, true)
	return m, nil
}

func (m *RootModel) clearPendingAttachment() {
	m.pendingAttachment = nil
	m.chat.ClearAttachment()
	m.statusBar.SetAttachStaged(false)
}

func fileNameSize(path string) (string, int64) {
	name := filepath.Base(path)
	var size int64
	if fi, err := os.Stat(path); err == nil {
		size = fi.Size()
	}
	return name, size
}
