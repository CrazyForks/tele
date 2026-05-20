package components_test

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestSpinner_FrameLength(t *testing.T) {
	s := components.NewSpinner()
	assert.Len(t, s.View(), 6)
}

func TestSpinner_TickAdvancesFrame(t *testing.T) {
	s := components.NewSpinner()
	first := s.View()
	s.Tick()
	assert.NotEqual(t, first, s.View())
}

func TestSpinner_FramesWrap(t *testing.T) {
	s := components.NewSpinner()
	first := s.View()
	for i := 0; i < 6; i++ {
		s.Tick()
	}
	assert.Equal(t, first, s.View())
}

func TestSpinner_AllFramesDistinct(t *testing.T) {
	s := components.NewSpinner()
	seen := make(map[string]struct{})
	for i := 0; i < 6; i++ {
		seen[s.View()] = struct{}{}
		s.Tick()
	}
	assert.Len(t, seen, 6)
}
