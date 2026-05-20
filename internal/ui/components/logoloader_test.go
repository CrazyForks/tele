package components_test

import (
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestLogoLoader_NarrowFallback(t *testing.T) {
	l := components.NewLogoLoader(20)
	assert.Equal(t, "tele", l.View())
}

func TestLogoLoader_NormalView_HasFiveRows(t *testing.T) {
	l := components.NewLogoLoader(80)
	lines := strings.Split(l.View(), "\n")
	nonEmpty := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			nonEmpty++
		}
	}
	assert.Equal(t, 5, nonEmpty)
}

func TestLogoLoader_StaticState_TickIsNoop(t *testing.T) {
	l := components.NewLogoLoader(80)
	// advance into sweep phase, then freeze
	for i := 0; i < 10; i++ {
		l.Tick()
	}
	l.SetState(components.LogoStateStatic)
	v1 := l.View()
	l.Tick()
	l.Tick()
	v2 := l.View()
	assert.Equal(t, v1, v2)
}

func TestLogoLoader_AnimatingState_TickChangesView(t *testing.T) {
	l := components.NewLogoLoader(80)
	// skip the pause phase (600ms / 80ms = 7.5 → 8 ticks)
	for i := 0; i < 8; i++ {
		l.Tick()
	}
	v1 := l.View()
	l.Tick()
	v2 := l.View()
	assert.NotEqual(t, v1, v2)
}
