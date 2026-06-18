package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sorokin-vladimir/tele/internal/store"
)

const maxComposerLines = 5

type Composer struct {
	ta                textarea.Model
	replyPreview      string
	focused           bool
	hasDarkBackground bool
	attachName        string
	attachSize        int64
	attachAs          store.MediaKind
	attachOn          bool
	attachToggle      bool
}

func NewComposer(width int) *Composer {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "> "
	ta.MaxHeight = maxComposerLines
	ta.DynamicHeight = true
	// Modifier+Enter combos (shift+enter, alt+enter) require a terminal that supports an extended
	// key protocol (Kitty keyboard protocol, or XTerm's modifyOtherKeys). Legacy terminals such as
	// macOS Terminal.app and MinTTY (Git for Windows) silently drop these keys, so neither binding
	// fires there. Both alternatives are registered so that whichever the terminal forwards is caught.
	// Lazygit has the same limitation and handles it identically — document the requirement and list
	// multiple fallbacks. Recommended terminals: Ghostty / iTerm2 (macOS), Windows Terminal (Windows),
	// kitty, WezTerm, Alacritty. tmux users need: set -g extended-keys on
	// See: https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Custom_Keybindings.md#terminal-compatibility
	// Issue: https://github.com/sorokin-vladimir/tele/issues/9#issuecomment-4600787928
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "shift+enter"))
	ta.KeyMap.Paste = key.NewBinding() // handled at root level via readClipboardCmd → tea.PasteMsg
	ta.CharLimit = 4096
	ta.SetWidth(width - 2)
	return &Composer{ta: ta}
}

func (c *Composer) SetWidth(w int) {
	c.ta.SetWidth(w - 2)
}

// Focus activates the composer cursor. Returns a blink Cmd that must be
// returned from the parent Update.
func (c *Composer) Focus() tea.Cmd {
	c.focused = true
	return c.ta.Focus()
}

func (c *Composer) Blur() {
	c.focused = false
	c.ta.Blur()
}

func (c *Composer) SetDarkBackground(isDark bool) { c.hasDarkBackground = isDark }

func (c *Composer) Value() string { return c.ta.Value() }

func (c *Composer) SetValue(v string) {
	c.ta.SetValue(v)
}

func (c *Composer) Reset() {
	c.ta.Reset()
	c.replyPreview = ""
}

func (c *Composer) SetReplyPreview(preview string) { c.replyPreview = preview }
func (c *Composer) ClearReplyPreview()             { c.replyPreview = "" }

// SetAttachment stages a file as a chip above the textarea. toggleable controls
// whether the "Send as: Photo/File" affordance is shown (image/video only).
func (c *Composer) SetAttachment(name string, size int64, sendAs store.MediaKind, toggleable bool) {
	c.attachName = name
	c.attachSize = size
	c.attachAs = sendAs
	c.attachToggle = toggleable
	c.attachOn = true
}

func (c *Composer) ClearAttachment() {
	c.attachOn = false
	c.attachName = ""
	c.attachToggle = false
}

func (c *Composer) HasAttachment() bool { return c.attachOn }

// attachmentLine renders the chip shown above the textarea, or "" if none.
func (c *Composer) attachmentLine() string {
	if !c.attachOn {
		return ""
	}
	line := fmt.Sprintf("📎 %s  %s", c.attachName, humanSize(c.attachSize))
	if c.attachToggle {
		photo, file := "Photo", "File"
		if c.attachAs == store.MediaPhoto {
			photo = "[Photo]"
		} else {
			file = "[File]"
		}
		line += fmt.Sprintf("   Send as: %s %s", photo, file)
	}
	return line
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// VisualHeight returns the total number of terminal rows that View() occupies:
// textarea lines + 2 border rows + preview lines (0 if no preview).
func (c *Composer) VisualHeight() int {
	h := c.ta.Height() + 2
	if c.replyPreview != "" {
		h += strings.Count(c.replyPreview, "\n") + 2
	}
	if c.attachOn {
		h++
	}
	return h
}

func (c *Composer) View() string {
	var parts []string
	if line := c.attachmentLine(); line != "" {
		parts = append(parts, line)
	}
	if c.replyPreview != "" {
		parts = append(parts, c.replyPreview, "")
	}
	parts = append(parts, c.ta.View())
	content := strings.Join(parts, "\n")

	// Do not set an explicit Width here: the textarea already pads every line
	// to its full inner width, producing a perfect content rectangle. Letting
	// lipgloss re-wrap that content via .Width() causes off-by-one overflow and
	// spurious soft-wraps at the wrap boundary (styled trailing spaces land on
	// the width edge), so we let the border size itself to the content instead.
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder())
	if c.focused {
		fg := lipgloss.LightDark(c.hasDarkBackground)(lipgloss.Color("19"), lipgloss.Color("12"))
		style = style.BorderForeground(fg)
	}
	return style.Render(content)
}

func (c *Composer) Init() tea.Cmd { return nil }

func (c *Composer) Update(msg tea.Msg) (*Composer, tea.Cmd) {
	var cmd tea.Cmd
	c.ta, cmd = c.ta.Update(msg)
	return c, cmd
}
