package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// testOwner stands in for core.Owner (#194). It holds the same pieces the real
// owner does — one state, one projection registry — and queues the deltas a
// mutation produces instead of pushing them down a channel, so a test can drain
// them into the model synchronously.
type testOwner struct {
	state  *state.State
	reg    *project.Registry
	queued []project.Delta
	// incoming records the events the real owner would publish alongside the
	// deltas: a message arriving in a chat the client is not showing.
	incoming []core.Incoming
}

func newTestOwner(st store.Store) *testOwner {
	o := &testOwner{state: state.New(st), reg: project.NewRegistry(st)}
	o.state.OnChange(func(chg state.Change) {
		if chg.Kind == state.ChangeTyping {
			o.queued = append(o.queued, o.reg.SetTyping(chg.ChatID, chg.Typing.Label())...)
			return
		}
		o.queued = append(o.queued, o.reg.Refresh()...)
	})
	return o
}

func (o *testOwner) Subscribe(w project.Window) project.SubID {
	id, ds := o.reg.Subscribe(w)
	o.queued = append(o.queued, ds...)
	return id
}

func (o *testOwner) MoveWindow(id project.SubID, w project.Window) {
	o.queued = append(o.queued, o.reg.MoveWindow(id, w)...)
}

func (o *testOwner) Unsubscribe(id project.SubID) { o.reg.Unsubscribe(id) }

func (o *testOwner) Refresh() { o.queued = append(o.queued, o.reg.Refresh()...) }

// drain feeds every queued delta and event into the model, in the order the
// bubbletea program would receive them, and returns the last command.
func (o *testOwner) drain(m ui.RootModel) (tea.Model, tea.Cmd) {
	var model tea.Model = m
	var cmd tea.Cmd
	for len(o.queued) > 0 || len(o.incoming) > 0 {
		deltas, events := o.queued, o.incoming
		o.queued, o.incoming = nil, nil
		for _, d := range deltas {
			model, cmd = model.Update(d)
		}
		for _, in := range events {
			model, cmd = model.Update(in)
		}
	}
	return model, cmd
}

// newRoot builds a model wired to a stand-in owner, the way app.Run wires the
// real one. Tests that pass no store get no owner, matching a model that has not
// reached the main screen.
func newRoot(client internaltg.Client, st store.Store, historyLimit int, verbose bool) ui.RootModel {
	m := ui.NewRootModel(client, st, historyLimit, verbose)
	if st != nil {
		m = m.WithOwner(newTestOwner(st))
	}
	return m
}

// applyEvent stands in for the owner's update loop: it applies a raw Telegram
// event to domain state and drains the resulting projection deltas into the
// model, exactly as core.Owner.RunUpdates plus the delta pump do in production.
//
// A no-op event (a read receipt that does not advance, an unchanged presence)
// produces no change and no delta, which is also what the owner does.
func applyEvent(t *testing.T, m ui.RootModel, st store.Store, evt store.Event) (tea.Model, tea.Cmd) {
	t.Helper()
	o, ok := m.Owner().(*testOwner)
	if !ok {
		// No owner attached: apply so the store advances, but nothing is
		// rendered — the same as a client that subscribed to nothing.
		state.Apply(state.New(st), evt)
		return m, nil
	}
	chg, applied := state.Apply(o.state, evt)
	if applied && chg.Kind == state.ChangeNewMessage && !chg.Message.IsOut &&
		chg.ChatID != m.CurrentChatID() {
		o.incoming = append(o.incoming, incomingFor(st, chg))
	}
	return o.drain(m)
}

// openChat opens a chat and drains the subscription's first delta, which is
// always a full Reset. In production that round trip is the bubbletea program
// delivering the owner's reply; a test has to make it happen itself.
func openChat(t *testing.T, m ui.RootModel, chatID int64, title string) ui.RootModel {
	t.Helper()
	next, _ := m.Update(screens.OpenChatMsg{ChatID: chatID, Title: title})
	m = next.(ui.RootModel)
	o, ok := m.Owner().(*testOwner)
	if !ok {
		return m
	}
	drained, _ := o.drain(m)
	return drained.(ui.RootModel)
}

// applyHistory stands in for a history backfill: it commits the store's current
// messages for a chat through state and drains the resulting chat:<id> delta
// into the model, the way core.Owner.backfill does in production. It replaces
// the old ChatHistoryMsg, which was the client applying a network reply itself.
func applyHistory(t *testing.T, m ui.RootModel, st store.Store, chatID int64) (tea.Model, tea.Cmd) {
	t.Helper()
	o, ok := m.Owner().(*testOwner)
	if !ok {
		return m, nil
	}
	o.state.ApplyHistory(chatID, st.Messages(chatID))
	return o.drain(m)
}

// incomingFor builds the event the owner publishes for a message that arrived
// in a chat the client is not showing.
func incomingFor(st store.Store, chg state.Change) core.Incoming {
	chat, _ := st.GetChat(chg.ChatID)
	return core.Incoming{
		ChatID:  chg.ChatID,
		Title:   chat.Title,
		Preview: chg.Message.Text,
		Notify:  !chat.IsMuted && !chat.IsArchived,
	}
}
