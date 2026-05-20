package components

import (
	"fmt"
	"math"
	"strings"
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

type cellKind int8

const (
	cellExterior cellKind = iota
	cellInterior
	cellBorder
)

type logoColorStop struct {
	pos     float64
	r, g, b uint8
}

var logoPalette = []logoColorStop{
	{0.00, 14, 18, 40},
	{0.30, 38, 65, 129},
	{0.60, 88, 136, 208},
	{0.85, 148, 192, 242},
	{1.00, 198, 228, 255},
}

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

func logoInterpolateColor(intensity float64) (r, g, b uint8) {
	pal := logoPalette
	for i := 0; i < len(pal)-1; i++ {
		lo, hi := pal[i], pal[i+1]
		if intensity >= lo.pos && intensity <= hi.pos {
			f := (intensity - lo.pos) / (hi.pos - lo.pos)
			return uint8(float64(lo.r) + f*float64(hi.r-lo.r)),
				uint8(float64(lo.g) + f*float64(hi.g-lo.g)),
				uint8(float64(lo.b) + f*float64(hi.b-lo.b))
		}
	}
	last := pal[len(pal)-1]
	return last.r, last.g, last.b
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
		return "tele"
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
				sb.WriteByte(' ')
			case cellBorder:
				displayCh := logoBorderWaveChar(ch, t)
				intensity := 0.14 + t*0.86
				rv, gv, bv := logoInterpolateColor(intensity)
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%s\x1b[0m", rv, gv, bv, string(displayCh))
			case cellInterior:
				ic := logoInteriorWaveChar(t)
				if ic == 0 {
					sb.WriteByte(' ')
				} else {
					rv, gv, bv := logoInterpolateColor(t)
					fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%s\x1b[0m", rv, gv, bv, string(ic))
				}
			}
		}
		if r < len(logoArt)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
