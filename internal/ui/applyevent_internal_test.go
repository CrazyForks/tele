package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// applyEventInternal is the in-package twin of applyEvent in the ui_test
// package: it stands in for the connection owner (#190), applying a raw
// Telegram event to domain state and feeding the resulting change to the model.
func applyEventInternal(t *testing.T, m RootModel, st store.Store, evt store.Event) (tea.Model, tea.Cmd) {
	t.Helper()
	chg, ok := state.Apply(state.New(st), evt)
	if !ok {
		return m, nil
	}
	return m.Update(chg)
}
