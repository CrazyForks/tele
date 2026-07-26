package state

import "github.com/sorokin-vladimir/tele/internal/store"

// State is the domain state of the account. Every mutation on the update path
// goes through one of its operations, and every operation funnels through
// commit, so there is exactly one place a state change can be observed from.
//
// Operations are named for what happened rather than for the field they set:
// ApplyIncoming, not AppendMessageAndBumpUnread. Callers describe events; the
// state decides what that means.
type State struct {
	st          store.Store
	subscribers []chan<- Change
}

func New(st store.Store) *State {
	return &State{st: st}
}

// Store exposes the underlying store for readers that have not yet moved to
// projections (#194). It is not a mutation path: writing through it bypasses
// commit and the change stream.
func (s *State) Store() store.Store { return s.st }

// Subscribe registers ch to receive every committed Change. Used by the owner
// to publish to its clients.
func (s *State) Subscribe(ch chan<- Change) {
	s.subscribers = append(s.subscribers, ch)
}

// commit is the single chokepoint every mutation passes through. Persistence is
// already handled write-through by the store, so today commit only publishes;
// it exists so that projections (#194) have one place to hook.
func (s *State) commit(c Change) {
	for _, ch := range s.subscribers {
		ch <- c
	}
}
