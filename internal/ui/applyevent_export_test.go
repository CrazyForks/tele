package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui"
)

// applyEvent stands in for the connection owner (#190): it applies a raw
// Telegram event to domain state and feeds the resulting change to the model.
// The UI no longer mutates on the update path, so a test that used to send a
// store.Event straight into Update must go through state first — exactly as
// core.Owner.RunUpdates does in production.
//
// A no-op event (a read receipt that does not advance, an unchanged presence)
// produces no change and leaves the model untouched, which is also what the
// owner does.
func applyEvent(t *testing.T, m ui.RootModel, st store.Store, evt store.Event) (tea.Model, tea.Cmd) {
	t.Helper()
	chg, ok := state.Apply(state.New(st), evt)
	if !ok {
		return m, nil
	}
	return m.Update(chg)
}
