package core

import (
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// store.Store is the reader every projection is built from. Asserted here so a
// change to either interface fails at compile time rather than at wiring time.
var _ project.Reader = (store.Store)(nil)

// Deltas is the stream every attached client consumes. Raw state changes do not
// reach a client: a client sees only the projections it subscribed to.
func (o *Owner) Deltas() <-chan project.Delta { return o.deltas }

// Subscribe registers a window. The subscription's first delta carries its
// current contents, which is what makes a resubscribe a full resync.
func (o *Owner) Subscribe(w project.Window) project.SubID {
	id, deltas := o.registry.Subscribe(w)
	o.publish(deltas)
	o.maybeBackfill(id, w)
	return id
}

// MoveWindow repositions a subscription. It returns immediately: over a socket a
// window move cannot be synchronous, so it is not synchronous here either.
func (o *Owner) MoveWindow(id project.SubID, w project.Window) {
	o.publish(o.registry.MoveWindow(id, w))
	o.maybeBackfill(id, w)
}

func (o *Owner) Unsubscribe(id project.SubID) { o.registry.Unsubscribe(id) }

// maybeBackfill fetches from Telegram when a chat window asked for more history
// than the store holds, so a client never has to know where data comes from.
func (o *Owner) maybeBackfill(id project.SubID, w project.Window) {
	cw, ok := w.(project.ChatWindow)
	if !ok || o.client == nil {
		return
	}
	if !needsBackfill(project.BuildChat(o.state.Store(), cw), cw) {
		return
	}
	go o.backfill(o.ctx, id, cw)
}

// needsBackfill reports that the store could not fill the window: it returned
// fewer messages than were asked for and has nothing older left to give. This is
// deliberately not HasOlder, which reports the opposite — that the store does
// hold more, so no fetch is needed.
func needsBackfill(c project.ChatContents, w project.ChatWindow) bool {
	return !c.HasOlder && len(c.Messages) < w.Before+w.After+1
}

// publishChange turns one applied domain change into whatever the current
// subscriptions actually need to hear. Every kind but typing is rebuilt from
// state; typing is ephemeral and has nothing persisted to rebuild from.
func (o *Owner) publishChange(chg state.Change) {
	if chg.Kind == state.ChangeTyping {
		o.publish(o.registry.SetTyping(chg.ChatID, chg.Typing.Label()))
		return
	}
	o.publish(o.registry.Refresh())
}

// publish drops deltas rather than blocking when a client is not draining:
// backpressure must never stall the owner's update loop. A dropped delta costs a
// stale window until the next change, and a resubscribe resyncs it.
func (o *Owner) publish(ds []project.Delta) {
	for _, d := range ds {
		select {
		case o.deltas <- d:
		default:
			o.log.Warn("projection delta dropped: client is not draining")
		}
	}
}
