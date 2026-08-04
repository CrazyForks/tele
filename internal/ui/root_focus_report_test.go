package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// The owner cannot see the screen. If the client reports opening a chat but
// never reports leaving it, the owner goes on believing an abandoned chat is in
// front of the user and silences it forever (#192).
func TestFocus_ReportedOnOpenAndOnClose(t *testing.T) {
	st := store.NewMemory()
	st.SetChat(domain.Chat{ID: 7, Title: "Alice"})
	m := newRootInternal(st, 50).WithScreen(ScreenMain)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	rm := model.(RootModel)
	stub := rm.owner.(*ownerStub)

	model, _ = rm.Update(screens.OpenChatMsg{ChatID: 7, Title: "Alice"})
	rm = model.(RootModel)
	if len(stub.focus) == 0 || stub.focus[len(stub.focus)-1] != 7 {
		t.Fatalf("opening a chat must report focus 7, got %v", stub.focus)
	}

	// Esc in normal mode closes the chat; the close path lives in handleMainKey
	// (internal/ui/root_keys.go).
	model, _ = rm.handleMainKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	rm = model.(RootModel)
	if rm.currentChatID != 0 {
		t.Fatal("Esc should have closed the chat")
	}
	if got := stub.focus[len(stub.focus)-1]; got != 0 {
		t.Fatalf("closing a chat must report focus 0, got %d (history %v)", got, stub.focus)
	}
}
