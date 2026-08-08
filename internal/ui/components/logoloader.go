package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// LogoTickMsg is sent by the root tick chain every 80ms while on the login screen.
type LogoTickMsg struct{}

// LogoLoaderState controls whether the wave animation runs.
type LogoLoaderState int

const (
	LogoStateAnimating LogoLoaderState = iota
	LogoStateStatic
)

const (
	logoHalfWidth = 6
	logoTickMs    = 80
	logoSweepMs   = 1800
	logoPauseMs   = 600
	logoCycleMs   = logoSweepMs + logoPauseMs
)

var logoArt = [5]string{
	"  _            _        ",
	" | |_    ___  | |   ___ ",
	" | __|  / _ \\ | |  / _ \\",
	" | |_  |  __/ | | |  __/",
	"  \\__|  \\___| |_|  \\___|",
}

// LogoLines returns the plain "tele" logo art, one string per row. It exists so
// output outside bubbletea (the farewell banner printed after the TUI exits)
// draws the same letters as the splash instead of keeping its own copy.
func LogoLines() []string {
	out := make([]string, len(logoArt))
	copy(out, logoArt[:])
	return out
}

type cellKind int8

const (
	cellExterior cellKind = iota
	cellInterior
	cellBorder
)

// LogoLoader renders the animated "tele" ASCII logo with a sweeping wave.
// Construct with NewLogoLoader. Call Tick() on each LogoTickMsg from root.
type LogoLoader struct {
	state   LogoLoaderState
	elapsed int // ms since cycle start
	width   int
	grid    [5][]cellKind
	cols    int
}

func NewLogoLoader(termWidth int) LogoLoader {
	l := LogoLoader{width: termWidth, cols: len(logoArt[0])}
	l.grid = buildLogoGrid()
	return l
}

func buildLogoGrid() [5][]cellKind {
	rows := len(logoArt)
	cols := len(logoArt[0])

	padded := [5][]rune{}
	for r, line := range logoArt {
		row := []rune(line)
		for len(row) < cols {
			row = append(row, ' ')
		}
		padded[r] = row
	}

	isBorder := func(ch rune) bool {
		return ch == '|' || ch == '_' || ch == '/' || ch == '\\'
	}

	ext := [5][]bool{}
	for r := range ext {
		ext[r] = make([]bool, cols)
	}
	type pt struct{ r, c int }
	q := []pt{}
	enq := func(r, c int) {
		if r < 0 || r >= rows || c < 0 || c >= cols {
			return
		}
		if ext[r][c] || isBorder(padded[r][c]) || padded[r][c] != ' ' {
			return
		}
		ext[r][c] = true
		q = append(q, pt{r, c})
	}
	for r := 0; r < rows; r++ {
		enq(r, 0)
		enq(r, cols-1)
	}
	for c := 0; c < cols; c++ {
		enq(0, c)
		enq(rows-1, c)
	}
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		enq(p.r-1, p.c)
		enq(p.r+1, p.c)
		enq(p.r, p.c-1)
		enq(p.r, p.c+1)
	}

	var grid [5][]cellKind
	for r := range logoArt {
		grid[r] = make([]cellKind, cols)
		for c := 0; c < cols; c++ {
			ch := padded[r][c]
			switch {
			case isBorder(ch):
				grid[r][c] = cellBorder
			case ext[r][c]:
				grid[r][c] = cellExterior
			default:
				grid[r][c] = cellInterior
			}
		}
	}
	return grid
}

// SetState freezes (LogoStateStatic) or resumes (LogoStateAnimating) the wave.
func (l *LogoLoader) SetState(s LogoLoaderState) { l.state = s }

// SetWidth updates the terminal width used to choose art vs narrow fallback.
func (l *LogoLoader) SetWidth(w int) { l.width = w }

// Tick advances the animation by one 80ms step. No-op when state is LogoStateStatic.
func (l *LogoLoader) Tick() {
	if l.state == LogoStateStatic {
		return
	}
	l.elapsed += logoTickMs
	if l.elapsed >= logoCycleMs {
		l.elapsed = 0
	}
}

func logoInterpolateColor(intensity float64, pal []theme.GradientStop) (r, g, b uint8) {
	// A theme sets the ramp, so its length is not fixed and an empty one has to
	// be survivable rather than a panic mid-frame.
	if len(pal) == 0 {
		return 0, 0, 0
	}
	for i := 0; i < len(pal)-1; i++ {
		lo, hi := pal[i], pal[i+1]
		if intensity >= lo.Pos && intensity <= hi.Pos {
			lr, lg, lb := rgb8(lo.Color)
			hr, hg, hb := rgb8(hi.Color)
			f := (intensity - lo.Pos) / (hi.Pos - lo.Pos)
			return uint8(float64(lr) + f*(float64(hr)-float64(lr))),
				uint8(float64(lg) + f*(float64(hg)-float64(lg))),
				uint8(float64(lb) + f*(float64(hb)-float64(lb)))
		}
	}
	return rgb8(pal[len(pal)-1].Color)
}

func logoBorderWaveChar(orig rune, t float64) rune {
	switch {
	case t >= 0.80:
		return '*'
	case t >= 0.65:
		return '+'
	case t >= 0.55:
		return ':'
	case t >= 0.45:
		return '.'
	case t >= 0.35:
		return '·'
	default:
		return orig
	}
}

func logoInteriorWaveChar(t float64) rune {
	switch {
	case t >= 0.80:
		return '*'
	case t >= 0.65:
		return '+'
	case t >= 0.55:
		return ':'
	case t >= 0.45:
		return '.'
	case t >= 0.10:
		return '·'
	default:
		return 0
	}
}

// View renders the logo with the current wave position applied.
// Returns "tele" for terminals narrower than 26 columns.
func (l LogoLoader) View() string {
	if l.width < 26 {
		return theme.S().Body.Render("tele")
	}

	var wavePos float64
	if l.elapsed < logoPauseMs {
		wavePos = float64(-logoHalfWidth - 1) // off-screen during pause
	} else {
		frac := float64(l.elapsed-logoPauseMs) / float64(logoSweepMs)
		wavePos = float64(-logoHalfWidth) + frac*float64(l.cols+2*logoHalfWidth)
	}

	runes := [5][]rune{}
	for r, line := range logoArt {
		row := []rune(line)
		for len(row) < l.cols {
			row = append(row, ' ')
		}
		runes[r] = row
	}

	pal := theme.T().LogoGradient

	// The logo writes its own SGR rather than going through a style: it repaints
	// every cell of a five-row banner on every animation frame, and a style per
	// cell would put lipgloss on that path. So the canvas is folded into the
	// escape it already writes, and its blank cells come from one prepared pad.
	bgSGR := ""
	if bg := theme.T().Background; !theme.IsNone(bg) {
		r, g, b, _ := bg.RGBA()
		bgSGR = fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
	}
	blank := theme.Pad(1)

	var sb strings.Builder
	for r := range logoArt {
		for c := 0; c < l.cols; c++ {
			dist := math.Abs(float64(c) - wavePos)
			var t float64
			if dist < float64(logoHalfWidth) {
				t = math.Pow(1-dist/float64(logoHalfWidth), 1.3)
			}
			ch := runes[r][c]
			switch l.grid[r][c] {
			case cellExterior:
				sb.WriteString(blank)
			case cellBorder:
				displayCh := logoBorderWaveChar(ch, t)
				intensity := 0.14 + t*0.86
				rv, gv, bv := logoInterpolateColor(intensity, pal)
				fmt.Fprintf(&sb, "%s\x1b[38;2;%d;%d;%dm%s\x1b[0m", bgSGR, rv, gv, bv, string(displayCh))
			case cellInterior:
				ic := logoInteriorWaveChar(t)
				if ic == 0 {
					sb.WriteString(blank)
				} else {
					rv, gv, bv := logoInterpolateColor(t, pal)
					fmt.Fprintf(&sb, "%s\x1b[38;2;%d;%d;%dm%s\x1b[0m", bgSGR, rv, gv, bv, string(ic))
				}
			}
		}
		if r < len(logoArt)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
