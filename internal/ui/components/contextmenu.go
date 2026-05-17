package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CloseContextMenuMsg is emitted when the context menu closes without an action.
type CloseContextMenuMsg struct{}

// DeleteMsgRequest is emitted when the user confirms deletion.
type DeleteMsgRequest struct {
	MsgID  int
	Revoke bool
}

type menuState int

const (
	stateMain menuState = iota
	stateDeleteSub
)

type menuItem struct {
	label string
	tag   string // "reply"|"react"|"edit"|"delete"|"delete_me"|"delete_revoke"|"cancel"|"sep"
}

// ContextMenu is a keyboard-navigable context menu overlaid on the chat view.
type ContextMenu struct {
	items  []menuItem
	cursor int
	state  menuState
	msgID  int
	isOut  bool
}

func NewContextMenu(msgID int, isOut bool) *ContextMenu {
	return &ContextMenu{
		items: mainItems(isOut),
		msgID: msgID,
		isOut: isOut,
	}
}

func mainItems(isOut bool) []menuItem {
	items := []menuItem{
		{label: "Reply", tag: "reply"},
		{label: "React", tag: "react"},
	}
	if isOut {
		items = append(items, menuItem{label: "Edit", tag: "edit"})
	}
	items = append(items, menuItem{label: "Delete", tag: "delete"})
	return items
}

func deleteSubItems() []menuItem {
	return []menuItem{
		{label: "For everyone", tag: "delete_revoke"},
		{label: "For me", tag: "delete_me"},
		{label: "─────────", tag: "sep"},
		{label: "Cancel", tag: "cancel"},
	}
}

// moveDown advances the cursor downward, wrapping around and skipping separators.
func (cm *ContextMenu) moveDown() {
	n := len(cm.items)
	for i := 1; i < n; i++ {
		next := (cm.cursor + i) % n
		if cm.items[next].tag != "sep" {
			cm.cursor = next
			return
		}
	}
}

// moveUp advances the cursor upward, wrapping around and skipping separators.
func (cm *ContextMenu) moveUp() {
	n := len(cm.items)
	for i := 1; i < n; i++ {
		prev := (cm.cursor - i + n) % n
		if cm.items[prev].tag != "sep" {
			cm.cursor = prev
			return
		}
	}
}

func (cm *ContextMenu) Update(msg tea.Msg) (*ContextMenu, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return cm, nil
	}
	switch km.String() {
	case "j", "down":
		cm.moveDown()
		return cm, nil
	case "k", "up":
		cm.moveUp()
		return cm, nil
	case "space":
		return nil, func() tea.Msg { return CloseContextMenuMsg{} }
	case "esc":
		if cm.state == stateDeleteSub {
			cm.state = stateMain
			cm.items = mainItems(cm.isOut)
			cm.cursor = 0
			return cm, nil
		}
		return nil, func() tea.Msg { return CloseContextMenuMsg{} }
	case "enter":
		return cm.execute()
	}
	return cm, nil
}

func (cm *ContextMenu) execute() (*ContextMenu, tea.Cmd) {
	tag := cm.items[cm.cursor].tag
	switch tag {
	case "reply", "react", "edit", "cancel":
		return nil, func() tea.Msg { return CloseContextMenuMsg{} }
	case "delete":
		if !cm.isOut {
			msgID := cm.msgID
			return nil, func() tea.Msg { return DeleteMsgRequest{MsgID: msgID, Revoke: false} }
		}
		cm.state = stateDeleteSub
		cm.items = deleteSubItems()
		cm.cursor = 0
		return cm, nil
	case "delete_me":
		msgID := cm.msgID
		return nil, func() tea.Msg { return DeleteMsgRequest{MsgID: msgID, Revoke: false} }
	case "delete_revoke":
		msgID := cm.msgID
		return nil, func() tea.Msg { return DeleteMsgRequest{MsgID: msgID, Revoke: true} }
	}
	return cm, nil
}

func (cm *ContextMenu) View() string {
	b := lipgloss.RoundedBorder()

	rows := make([]string, len(cm.items))
	for i, item := range cm.items {
		if item.tag == "sep" {
			rows[i] = "  " + item.label
		} else if i == cm.cursor {
			rows[i] = " ▸ " + item.label
		} else {
			rows[i] = "   " + item.label
		}
	}

	innerW := 0
	for _, r := range rows {
		if w := lipgloss.Width(r); w > innerW {
			innerW = w
		}
	}
	innerW++ // one space of right padding

	top := b.TopLeft + strings.Repeat(b.Top, innerW) + b.TopRight
	bot := b.BottomLeft + strings.Repeat(b.Bottom, innerW) + b.BottomRight

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, top)
	for _, r := range rows {
		pad := strings.Repeat(" ", innerW-lipgloss.Width(r))
		lines = append(lines, b.Left+r+pad+b.Right)
	}
	lines = append(lines, bot)
	return strings.Join(lines, "\n")
}
