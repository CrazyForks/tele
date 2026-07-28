package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// ownerStub is the in-package twin of the ui_test testOwner: one state and one
// projection registry, queueing the deltas a mutation produces so a test can
// drain them into the model synchronously.
type ownerStub struct {
	state    *state.State
	reg      *project.Registry
	queued   []project.Delta
	incoming []core.Incoming
	typing   []core.Typing
}

func newOwnerStub(st store.Store) *ownerStub {
	o := &ownerStub{state: state.New(st), reg: project.NewRegistry(st)}
	o.state.OnChange(func(chg state.Change) {
		if chg.Kind == state.ChangeTyping {
			o.typing = append(o.typing, core.Typing{ChatID: chg.ChatID, Label: chg.Typing.Label()})
			return
		}
		o.queued = append(o.queued, o.reg.Refresh()...)
	})
	return o
}

func (o *ownerStub) Subscribe(w project.Window) project.SubID {
	id, ds := o.reg.Subscribe(w)
	o.queued = append(o.queued, ds...)
	return id
}

func (o *ownerStub) MoveWindow(id project.SubID, w project.Window) {
	o.queued = append(o.queued, o.reg.MoveWindow(id, w)...)
}

func (o *ownerStub) Unsubscribe(id project.SubID) { o.reg.Unsubscribe(id) }

func (o *ownerStub) Refresh() { o.queued = append(o.queued, o.reg.Refresh()...) }

func (o *ownerStub) drain(m RootModel) (tea.Model, tea.Cmd) {
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

// newRootInternal builds a model wired to the stub owner, as app.Run wires the
// real one.
func newRootInternal(st store.Store, historyLimit int) RootModel {
	m := NewRootModel(nil, st, historyLimit, false)
	if st != nil {
		m = m.WithOwner(newOwnerStub(st))
	}
	return m
}

// applyEventInternal is the in-package twin of applyEvent in the ui_test
// package: it applies a raw Telegram event to domain state and drains the
// resulting projection deltas into the model, as the owner's update loop and the
// delta pump do in production.
func applyEventInternal(t *testing.T, m RootModel, st store.Store, evt store.Event) (tea.Model, tea.Cmd) {
	t.Helper()
	o, ok := m.owner.(*ownerStub)
	if !ok {
		state.Apply(state.New(st), evt)
		return m, nil
	}
	chg, applied := state.Apply(o.state, evt)
	if applied && chg.Kind == state.ChangeNewMessage && !chg.Message.IsOut &&
		chg.ChatID != m.currentChatID {
		o.incoming = append(o.incoming, incomingLike(m, st, chg))
	}
	return o.drain(m)
}

// incomingLike reproduces core.Owner.publishIncoming: the same freshness and
// mute gate, and the same privacy rule on the preview. Keeping the two in step
// is what makes a UI test about notifications mean anything.
func incomingLike(m RootModel, st store.Store, chg state.Change) core.Incoming {
	chat, _ := st.GetChat(chg.ChatID)
	preview := chg.Message.Text
	if m.cfg != nil && !m.cfg.UI.NotificationPreview {
		preview = "New message"
	}
	return core.Incoming{
		ChatID:  chg.ChatID,
		Title:   chat.Title,
		Preview: preview,
		Notify:  store.Notifiable(st, chg.ChatID, m.currentChatID, chg.Message.Date, time.Now()),
	}
}
