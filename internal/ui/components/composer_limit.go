package components

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// flashDuration is how long the composer border stays red after a limit event.
const flashDuration = 300 * time.Millisecond

// ComposerLimitKind distinguishes the ways a draft meets Telegram's length
// limit, so the caller can word each one appropriately.
type ComposerLimitKind int

const (
	// ComposerLimitTyping: a keystroke was refused at the limit.
	ComposerLimitTyping ComposerLimitKind = iota
	// ComposerLimitPaste: a paste was applied in full and took the draft over.
	ComposerLimitPaste
	// ComposerLimitOver: the draft is already over the limit (send refused, or a
	// file was attached to a draft longer than a caption may be).
	ComposerLimitOver
)

// ComposerLimitMsg reports a limit event. The composer does not own user-facing
// wording (see Composer.SetPlaceholder); root renders these fields into text.
type ComposerLimitMsg struct {
	Kind    ComposerLimitKind
	Limit   int  // the limit in force, in UTF-16 code units
	Caption bool // true when the composer is the caption field
}

// ComposerFlashOffMsg ends a border flash. The serial guards against a stale
// tick clearing a flash raised after it.
type ComposerFlashOffMsg struct{ Serial int }

// SignalLimit flashes the border and, once per over-limit episode, emits a
// ComposerLimitMsg for the caller to surface. The episode flag re-arms once the
// draft is back within the limit, so holding a key at the limit cannot queue one
// toast per keystroke.
func (c *Composer) SignalLimit(kind ComposerLimitKind) tea.Cmd {
	c.flash = true
	c.flashSerial++
	serial := c.flashSerial

	cmds := []tea.Cmd{tea.Tick(flashDuration, func(time.Time) tea.Msg {
		return ComposerFlashOffMsg{Serial: serial}
	})}
	if !c.warned {
		c.warned = true
		limit, caption := c.limit(), len(c.attachments) > 0
		cmds = append(cmds, func() tea.Msg {
			return ComposerLimitMsg{Kind: kind, Limit: limit, Caption: caption}
		})
	}
	return tea.Batch(cmds...)
}

// FlashActive reports whether the border is currently flashing.
func (c *Composer) FlashActive() bool { return c.flash }

// FlashSerialForTest returns the current flash serial (test accessor).
func (c *Composer) FlashSerialForTest() int { return c.flashSerial }
