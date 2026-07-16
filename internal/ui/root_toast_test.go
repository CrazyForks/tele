package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// mainScreenModel builds a sized main-screen RootModel for toast tests.
func mainScreenModel() RootModel {
	m := NewRootModel(nil, nil, 50, false).WithScreen(ScreenMain)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return model.(RootModel)
}

// drainClearSerial extracts the ClearStatusErrMsg serial from a scheduled cmd.
func drainClearSerial(t *testing.T, cmd tea.Cmd) int {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a tick command")
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			if cs, ok := c().(ClearStatusErrMsg); ok {
				return cs.Serial
			}
		}
		t.Fatal("no ClearStatusErrMsg in batch")
	}
	if cs, ok := msg.(ClearStatusErrMsg); ok {
		return cs.Serial
	}
	t.Fatalf("unexpected msg %T", msg)
	return 0
}

func TestStatusErr_RendersInToastNotStatusBar(t *testing.T) {
	m := mainScreenModel()
	model, _ := m.Update(StatusErrMsg{Text: "connection lost", Sev: components.SeverityError})
	rm := model.(RootModel)
	if rm.toasts.Empty() {
		t.Fatal("StatusErrMsg should add a toast")
	}
	view := rm.View().Content
	if !strings.Contains(view, "connection lost") {
		t.Fatalf("toast text not in view:\n%s", view)
	}
}

func TestClearStatusErr_DismissesToast(t *testing.T) {
	m := mainScreenModel()
	model, cmd := m.Update(StatusErrMsg{Text: "boom", Sev: components.SeverityError})
	rm := model.(RootModel)
	serial := drainClearSerial(t, cmd)
	model2, _ := rm.Update(ClearStatusErrMsg{Serial: serial})
	rm2 := model2.(RootModel)
	if !rm2.toasts.Empty() {
		t.Fatal("ClearStatusErrMsg should dismiss the toast")
	}
}

func TestDismissToastAction_ClosesTopToast(t *testing.T) {
	m := mainScreenModel()
	model, _ := m.Update(StatusErrMsg{Text: "boom", Sev: components.SeverityError})
	rm := model.(RootModel)
	model2, _ := rm.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	rm2 := model2.(RootModel)
	if !rm2.toasts.Empty() {
		t.Fatal("ctrl+x should dismiss the top toast")
	}
}

func TestMouseClick_ToastActionEmitsMsg(t *testing.T) {
	rm := mainScreenModel()
	// A toast carrying a clickable action.
	rm.toasts.Add(components.ToastError, "click me",
		components.ToastAction{Label: "close", Key: "x", Msg: ClearStatusErrMsg{Serial: 0}})

	rects := rm.toasts.HitTestRects()
	if len(rects) == 0 {
		t.Fatal("expected an action region")
	}
	r := rects[0].Rect
	cx, cy := r.Left+r.Width/2, r.Top+r.Height/2
	_, cmd := rm.handleMouseClick(tea.Mouse{X: cx, Y: cy, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("clicking an action should return a command")
	}
}

func TestChatLoadErr_ToastHasRetryAction(t *testing.T) {
	m := mainScreenModel()
	m.currentChatID = 42
	model, _ := m.Update(chatLoadErrMsg{chatID: 42, text: "load failed"})
	rm := model.(RootModel)
	found := false
	for _, r := range rm.toasts.HitTestRects() {
		if _, ok := r.Msg.(retryChatLoadMsg); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("chat-load error toast must carry a retry action")
	}
}

func TestParseToastZone(t *testing.T) {
	if parseToastZone("top-right") != components.ZoneTopRight {
		t.Fatal("top-right")
	}
	if parseToastZone("bottom-left") != components.ZoneBottomLeft {
		t.Fatal("bottom-left")
	}
	if parseToastZone("garbage") != components.ZoneBottomRight {
		t.Fatal("unknown must default to bottom-right")
	}
}
