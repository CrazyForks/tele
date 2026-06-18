package screens_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/ui/keys"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

func TestFilePickerNavigateAndSelect(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pic.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := screens.NewFilePickerModel(dir, 60, 24, keys.DefaultKeyMap())

	// Directories sort above files; ".." is row 0, then "sub", then "pic.jpg".
	names := []string{}
	for _, e := range m.Entries() {
		names = append(names, e.Name())
	}
	if len(names) < 3 || names[0] != ".." || names[1] != "sub" || names[2] != "pic.jpg" {
		t.Fatalf("entry order = %v", names)
	}

	// Filter to the file by typing.
	for _, r := range "pic" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if m.CurrentName() != "pic.jpg" {
		t.Fatalf("after filter, current = %q", m.CurrentName())
	}

	// Enter selects the file.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on file produced no cmd")
	}
	msg := cmd()
	sel, ok := msg.(screens.FileSelectedMsg)
	if !ok {
		t.Fatalf("got %T, want FileSelectedMsg", msg)
	}
	if sel.Path != filepath.Join(dir, "pic.jpg") {
		t.Fatalf("selected path = %q", sel.Path)
	}
}

func TestFilePickerDescendAndAscend(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := screens.NewFilePickerModel(dir, 60, 24, keys.DefaultKeyMap())

	// Move cursor to "sub" (row 1) and enter to descend.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Dir() != sub {
		t.Fatalf("after descend, dir = %q, want %q", m.Dir(), sub)
	}

	// Backspace ascends.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.Dir() != dir {
		t.Fatalf("after ascend, dir = %q, want %q", m.Dir(), dir)
	}
}

func TestFilePickerPastePathSelectsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := screens.NewFilePickerModel(dir, 60, 24, keys.DefaultKeyMap())
	_, cmd := m.Update(tea.PasteMsg{Content: path})
	if cmd == nil {
		t.Fatal("paste of a file path produced no cmd")
	}
	sel, ok := cmd().(screens.FileSelectedMsg)
	if !ok {
		t.Fatalf("got %T, want FileSelectedMsg", cmd())
	}
	if sel.Path != path {
		t.Fatalf("selected path = %q", sel.Path)
	}
}

func TestFilePickerScrollsToKeepCursorVisible(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 30; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i/26))+string(rune('a'+i%26)))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := screens.NewFilePickerModel(dir, 60, 24, keys.DefaultKeyMap())

	// The cursor starts at the top; the last entry is off-screen initially.
	last := m.Entries()[len(m.Entries())-1].Name()
	if strings.Contains(m.View(), last) {
		t.Fatalf("last entry %q unexpectedly visible before scrolling", last)
	}

	// Move the cursor down to the last entry; the view must scroll to show it.
	for i := 0; i < len(m.Entries()); i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if !strings.Contains(m.View(), last) {
		t.Fatalf("cursor entry %q not visible after scrolling:\n%s", last, m.View())
	}
}

func TestFilePickerEsc(t *testing.T) {
	m := screens.NewFilePickerModel(t.TempDir(), 60, 24, keys.DefaultKeyMap())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc produced no cmd")
	}
	if _, ok := cmd().(screens.CloseFilePickerMsg); !ok {
		t.Fatalf("esc did not emit CloseFilePickerMsg, got %T", cmd())
	}
}
