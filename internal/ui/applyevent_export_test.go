package ui_test

import (
	"testing"
	"time"

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
	typing   []core.Typing
	// moves records the windows the client asked for, so a test can assert that
	// scrolling repositions a window rather than fetching anything itself.
	moves []project.Window
}

func newTestOwner(st store.Store) *testOwner {
	o := &testOwner{state: state.New(st), reg: project.NewRegistry(st)}
	o.state.OnChange(func(chg state.Change) {
		if chg.Kind == state.ChangeTyping {
			o.typing = append(o.typing, core.Typing{ChatID: chg.ChatID, Label: chg.Typing.Label()})
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
	o.moves = append(o.moves, w)
	o.queued = append(o.queued, o.reg.MoveWindow(id, w)...)
}

// lastChatWindow returns the most recent chat window the client asked for.
func (o *testOwner) lastChatWindow() (project.ChatWindow, bool) {
	for i := len(o.moves) - 1; i >= 0; i-- {
		if w, ok := o.moves[i].(project.ChatWindow); ok {
			return w, true
		}
	}
	return project.ChatWindow{}, false
}

// lastChatListWindow returns the most recent chatlist window the client asked for.
func (o *testOwner) lastChatListWindow() (project.ChatListWindow, bool) {
	for i := len(o.moves) - 1; i >= 0; i-- {
		if w, ok := o.moves[i].(project.ChatListWindow); ok {
			return w, true
		}
	}
	return project.ChatListWindow{}, false
}

func (o *testOwner) Unsubscribe(id project.SubID) { o.reg.Unsubscribe(id) }

func (o *testOwner) Refresh() { o.queued = append(o.queued, o.reg.Refresh()...) }

// drain feeds every queued delta and event into the model, in the order the
// bubbletea program would receive them, and returns the last command.
func (o *testOwner) drain(m ui.RootModel) (tea.Model, tea.Cmd) {
	var model tea.Model = m
	var cmd tea.Cmd
	for len(o.queued) > 0 || len(o.incoming) > 0 || len(o.typing) > 0 {
		deltas, events, typing := o.queued, o.incoming, o.typing
		o.queued, o.incoming, o.typing = nil, nil, nil
		for _, d := range deltas {
			model, cmd = model.Update(d)
		}
		for _, in := range events {
			model, cmd = model.Update(in)
		}
		for _, tp := range typing {
			model, cmd = model.Update(tp)
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
		o.incoming = append(o.incoming, incomingFor(st, m, chg))
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

// drainOwner feeds whatever the owner has queued into the model. Anything that
// subscribes or moves a window queues a delta the bubbletea program would
// deliver; a test has to deliver it itself.
func drainOwner(t *testing.T, m ui.RootModel) ui.RootModel {
	t.Helper()
	o, ok := m.Owner().(*testOwner)
	if !ok {
		return m
	}
	drained, _ := o.drain(m)
	return drained.(ui.RootModel)
}

// toMain reaches the main screen, where the chat list first has a size and
// subscribes, and drains the subscription's opening Reset.
func toMain(t *testing.T, m ui.RootModel) ui.RootModel {
	t.Helper()
	next, _ := m.Update(screens.TransitionToMainMsg{})
	return drainOwner(t, next.(ui.RootModel))
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

// incomingFor reproduces core.Owner.publishIncoming: the same freshness and mute
// gate the owner applies before telling a client anything arrived.
func incomingFor(st store.Store, m ui.RootModel, chg state.Change) core.Incoming {
	chat, _ := st.GetChat(chg.ChatID)
	return core.Incoming{
		ChatID:  chg.ChatID,
		Title:   chat.Title,
		Preview: chg.Message.Text,
		Notify:  store.Notifiable(st, chg.ChatID, m.CurrentChatID(), chg.Message.Date, time.Now()),
	}
}
