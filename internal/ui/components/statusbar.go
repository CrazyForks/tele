package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
)

var (
	barBg    = lipgloss.Color("236")
	barFg    = lipgloss.Color("252")
	barStyle = lipgloss.NewStyle().Background(barBg).Foreground(barFg)

	barSepStyle = lipgloss.NewStyle().Background(barBg).Foreground(lipgloss.Color("240"))

	modeBase   = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("231"))
	normalMode = modeBase.Background(lipgloss.Color("33")) // blue
	insertMode = modeBase.Background(lipgloss.Color("35")) // green
)

// Severity classifies a transient status-bar message.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

type StatusBar struct {
	width      int
	mode       keys.VimMode
	status     string
	verbose    bool
	lastKey    string
	activePane string
	keyMap     keys.KeyMap
	errText    string
	errSev     Severity
	errSerial  int
	// attachStaged is true while a file is staged in the composer (chip shown);
	// pickerOpen is true while the file-picker overlay is open. Both drive hints.
	attachStaged bool
	pickerOpen   bool
}

func NewStatusBar(width int) *StatusBar {
	return &StatusBar{width: width, mode: keys.ModeNormal}
}

func (sb *StatusBar) SetWidth(w int)           { sb.width = w }
func (sb *StatusBar) SetMode(m keys.VimMode)   { sb.mode = m }
func (sb *StatusBar) SetStatus(s string)       { sb.status = s }
func (sb *StatusBar) SetVerbose(v bool)        { sb.verbose = v }
func (sb *StatusBar) SetLastKey(k string)      { sb.lastKey = k }
func (sb *StatusBar) SetActivePane(p string)   { sb.activePane = p }
func (sb *StatusBar) SetKeyMap(km keys.KeyMap) { sb.keyMap = km }
func (sb *StatusBar) SetAttachStaged(v bool)   { sb.attachStaged = v }
func (sb *StatusBar) SetPickerOpen(v bool)     { sb.pickerOpen = v }

// SetError shows a transient, severity-tagged message and returns the serial
// identifying it, so a later ClearError only clears this exact message.
func (sb *StatusBar) SetError(text string, sev Severity) int {
	sb.errSerial++
	sb.errText = text
	sb.errSev = sev
	return sb.errSerial
}

// ClearError clears the error only when serial matches the current one, so a
// stale auto-clear timer cannot wipe a newer error.
func (sb *StatusBar) ClearError(serial int) {
	if serial == sb.errSerial {
		sb.errText = ""
	}
}

func (sb *StatusBar) View() string {
	modeStyle := normalMode
	label := "NORMAL"
	if sb.mode == keys.ModeInsert {
		modeStyle = insertMode
		label = "INSERT"
	}

	segs := []string{modeStyle.Render(label)}

	if sb.errText != "" {
		segs = append(segs, errStyle(sb.errSev).Render(sb.errText))
	} else if sb.status != "" {
		segs = append(segs, barStyle.Render(sb.status))
	}
	if h := sb.hints(); h != "" {
		segs = append(segs, barStyle.Render(h))
	}
	if sb.verbose {
		segs = append(segs, barStyle.Render(fmt.Sprintf("pane:%s key:%s", sb.activePane, sb.lastKey)))
	}

	sep := barSepStyle.Render(" │ ")
	return barStyle.Width(sb.width).Render(strings.Join(segs, sep))
}

func errStyle(sev Severity) lipgloss.Style {
	base := lipgloss.NewStyle().Background(barBg).Bold(true)
	switch sev {
	case SeverityError:
		return base.Foreground(lipgloss.Color("203")) // red
	case SeverityWarning:
		return base.Foreground(lipgloss.Color("214")) // amber
	default:
		return base.Foreground(lipgloss.Color("75")) // blue/info
	}
}

func (sb *StatusBar) hints() string {
	if sb.keyMap == nil {
		return ""
	}
	switch {
	case sb.pickerOpen:
		confirm := sb.keyMap.KeyFor(keys.ContextFilePicker, keys.ActionConfirm)
		cancel := sb.keyMap.KeyFor(keys.ContextFilePicker, keys.ActionCancel)
		return joinHints(
			"type -> filter",
			hintKey(confirm, "open/select"),
			hintKey(cancel, "cancel"),
		)
	case sb.activePane == "chat" && sb.mode == keys.ModeInsert && sb.attachStaged:
		send := sb.keyMap.KeyFor(keys.ContextComposer, keys.ActionConfirm)
		toggle := sb.keyMap.KeyFor(keys.ContextComposer, keys.ActionToggleSendAs)
		normal := sb.keyMap.KeyFor(keys.ContextComposer, keys.ActionNormal)
		return joinHints(
			hintKey(send, "send"),
			hintKey(toggle, "photo/file"),
			hintKey(normal, "normal"),
		)
	case sb.activePane == "chat" && sb.attachStaged:
		write := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionInsert)
		drop := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionCancelUpload)
		return joinHints(
			hintKey(write, "caption"),
			hintKey(drop, "drop file"),
		)
	case sb.activePane == "folders":
		down := sb.keyMap.KeyFor(keys.ContextFolders, keys.ActionDown)
		up := sb.keyMap.KeyFor(keys.ContextFolders, keys.ActionUp)
		sel := sb.keyMap.KeyFor(keys.ContextFolders, keys.ActionConfirm)
		quit := sb.keyMap.KeyFor(keys.ContextGlobal, keys.ActionQuit)
		return joinHints(
			hintNav(down, up, "move"),
			hintKey(sel, "select"),
			hintKey(quit, "quit"),
		)
	case sb.activePane == "chat" && sb.mode == keys.ModeInsert:
		send := sb.keyMap.KeyFor(keys.ContextComposer, keys.ActionConfirm)
		normal := sb.keyMap.KeyFor(keys.ContextComposer, keys.ActionNormal)
		return joinHints(hintKey(send, "send"), hintKey(normal, "normal"))
	case sb.activePane == "chat":
		down := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionDown)
		up := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionUp)
		curDown := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionCursorDown)
		curUp := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionCursorUp)
		write := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionInsert)
		attach := sb.keyMap.KeyFor(keys.ContextChat, keys.ActionAttach)
		quit := sb.keyMap.KeyFor(keys.ContextGlobal, keys.ActionQuit)
		return joinHints(
			hintNav(down, up, "scroll"),
			hintNav(curDown, curUp, "select"),
			hintKey(write, "write"),
			hintKey(attach, "attach"),
			hintKey(quit, "quit"),
		)
	case sb.activePane == "chatlist":
		down := sb.keyMap.KeyFor(keys.ContextChatList, keys.ActionDown)
		up := sb.keyMap.KeyFor(keys.ContextChatList, keys.ActionUp)
		open := sb.keyMap.KeyFor(keys.ContextChatList, keys.ActionConfirm)
		search := sb.keyMap.KeyFor(keys.ContextChatList, keys.ActionSearch)
		quit := sb.keyMap.KeyFor(keys.ContextGlobal, keys.ActionQuit)
		return joinHints(
			hintNav(down, up, "move"),
			hintKey(open, "open"),
			hintKey(search, "search"),
			hintKey(quit, "quit"),
		)
	}
	return ""
}

func hintKey(key, desc string) string {
	if key == "" {
		return ""
	}
	return key + " -> " + desc
}

func hintNav(downKey, upKey, desc string) string {
	if downKey == "" && upKey == "" {
		return ""
	}
	combo := downKey + "/" + upKey
	// Collapse a shared modifier prefix: "ctrl+j"/"ctrl+k" -> "ctrl+j/k".
	if i := strings.LastIndex(downKey, "+"); i >= 0 {
		prefix := downKey[:i+1]
		if strings.HasPrefix(upKey, prefix) {
			combo = downKey + "/" + upKey[len(prefix):]
		}
	}
	return combo + " -> " + desc
}

func joinHints(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}
