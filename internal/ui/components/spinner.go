package components

// SpinnerTickMsg is sent by the root tick chain every 150ms while on the main screen.
type SpinnerTickMsg struct{}

var spinnerFrames = [6]string{"[=   ]", "[==  ]", "[=== ]", "[ ===]", "[  ==]", "[   =]"}

// Spinner is a ping-pong bar. Call Tick() on each SpinnerTickMsg. Call View() to render.
type Spinner struct {
	frame int
}

func NewSpinner() Spinner { return Spinner{} }

// Tick advances the spinner by one frame.
func (s *Spinner) Tick() {
	s.frame = (s.frame + 1) % len(spinnerFrames)
}

// View returns the current frame string (always 6 chars wide).
func (s Spinner) View() string {
	return spinnerFrames[s.frame]
}
