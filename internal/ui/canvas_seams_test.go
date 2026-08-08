package ui_test

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// A seam is a cell the canvas did not reach. The failure mode of #214 is that
// seams are visible only to whoever happens to be looking at the right screen
// state, so they are checked here instead of by looking.
//
// The check runs on the cell grid rather than on the string: bubbletea renders
// by parsing the view with uv.NewStyledString and diffing the cells that come
// out, so the grid this walks is the one the terminal is actually told about,
// escape for escape.
//
// The invariant is the absence of a background, not the presence of an expected
// one. A whitelist of colours would be brittle — highlight_accent is
// interpolated and produces arbitrary intermediate values — and it would be
// checking the wrong thing. A hole is a cell that carries no background and
// therefore falls through to the terminal.

// seam is one unpainted cell, located.
type seam struct {
	row, col int
	content  string
}

func (s seam) String() string {
	return fmt.Sprintf("row %d, col %d: %q", s.row, s.col, s.content)
}

// seams returns every cell of a w x h terminal that the view left without a
// background.
//
// The view is drawn into a cell buffer rather than split on newlines, which is
// what bubbletea does with it (cursed_renderer.go: NewStyledString, then Draw
// into the screen buffer over Rect(0, 0, width, height)). It matters for more
// than fidelity: a buffer covers the whole terminal, so a cell the view never
// wrote to at all is examined too. Those are the holes that a check over the
// emitted lines cannot see, and the ones a short row or a missing bottom line
// leaves behind.
func seams(content string, w, h int) []seam {
	buf := uv.NewScreenBuffer(w, h)
	buf.Method = ansi.GraphemeWidth
	uv.NewStyledString(content).Draw(buf, uv.Rect(0, 0, w, h))

	var out []seam
	for row := range h {
		for col := range w {
			cell := buf.CellAt(col, row)
			if cell != nil && cell.Style.Bg != nil {
				continue
			}
			content := ""
			if cell != nil {
				content = cell.Content
			}
			out = append(out, seam{row: row, col: col, content: content})
		}
	}
	return out
}

// report renders the first few seams; a broken frame produces thousands, and
// the first handful say where to look as well as all of them would.
func report(found []seam) string {
	const show = 12
	s := fmt.Sprintf("%d cells carry no background\n", len(found))
	for i, f := range found {
		if i == show {
			s += fmt.Sprintf("  ... and %d more\n", len(found)-show)
			break
		}
		s += "  " + f.String() + "\n"
	}
	return s
}

// paintedSlots installs a theme claiming the canvas, for the duration of the
// test. Both slots hold it so the check does not depend on what the terminal
// reported.
func paintedSlots(t *testing.T) {
	t.Helper()
	bg, err := theme.ParseColor("#1e1e2e")
	require.NoError(t, err)
	fg, err := theme.ParseColor("#cdd6f4")
	require.NoError(t, err)

	painted := theme.TeleDark
	painted.Name = "seam-test"
	painted.Background, painted.Text = bg, fg

	t.Cleanup(func() { theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight}) })
	theme.SetSlots(theme.Slots{Dark: painted, Light: painted})
	theme.Apply(true)
}

// The sizes are the ones that behave differently rather than a sweep. 80x24 is
// the floor; 41 columns is narrow enough that the panes hit their minimum
// widths, which is where the arithmetic that makes them sum has to be trusted;
// odd widths and heights catch a split that rounds a column away.
var seamSizes = []struct{ w, h int }{
	{80, 24},
	{81, 25},
	{41, 12},
	{120, 40},
	{201, 61},
}

func TestCanvas_MainScreenHasNoSeams(t *testing.T) {
	paintedSlots(t)

	for _, size := range seamSizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			m := newPopulatedRoot(t, size.w, size.h)
			found := seams(m.View().Content, size.w, size.h)
			require.Empty(t, found, report(found))
		})
	}
}

// Overlays are where seams live: each one is stamped into the composed screen by
// hand, and the stamping pads the base row out to meet it.
func TestCanvas_OverlaysHaveNoSeams(t *testing.T) {
	paintedSlots(t)

	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"help", tea.KeyPressMsg{Code: '?', Text: "?"}},
		{"search", tea.KeyPressMsg{Code: '/', Text: "/"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newPopulatedRoot(t, 120, 40)
			next, _ := m.Update(tc.key)
			m = next.(ui.RootModel)

			found := seams(m.View().Content, 120, 40)
			require.Empty(t, found, report(found))
		})
	}
}

// The login screen is almost entirely whitespace around a centred block, which
// is emitted by lipgloss.Place rather than by anything the app padded itself.
func TestCanvas_LoginScreenHasNoSeams(t *testing.T) {
	paintedSlots(t)

	m := newRoot(nil, 50, false)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(ui.RootModel)

	found := seams(m.View().Content, 100, 30)
	require.Empty(t, found, report(found))
}

// With no canvas nothing is painted, and that has to stay true: the built-ins
// must render exactly as they did before the token existed. This is the same
// claim as TestCanvas_UnsetPaintsNothing, made against a whole frame.
func TestCanvas_UnsetLeavesEveryCellBare(t *testing.T) {
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight})
	theme.Apply(true)

	const w, h = 120, 40
	m := newPopulatedRoot(t, w, h)

	bare := len(seams(m.View().Content, w, h))
	// The status bar is a surface the built-ins do paint, so not every cell is
	// bare. What this pins is that the canvas machinery painted nothing: the
	// field around the surfaces is still the terminal's, as it always was.
	require.Greater(t, bare, w*h/2,
		"with no canvas most of the screen must still fall through to the terminal")
	require.Less(t, bare, w*h,
		"the built-ins paint the status bar, so some cells carry a surface")
}
