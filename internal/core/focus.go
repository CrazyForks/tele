package core

import "sync"

// clientID identifies one attached client for the lifetime of its attachment.
type clientID uint64

// focusRegistry records which chat each attached client is looking at. It is the
// notification policy's only view of clients: nothing else about a client
// affects a decision.
//
// It is a set rather than a single value for two reasons. The owner may have
// several clients attached (#183), and a client that goes away has to take its
// focus with it — otherwise a crashed client silences its last-open chat
// permanently.
type focusRegistry struct {
	mu   sync.Mutex
	next clientID
	on   map[clientID]int64 // 0 means the client has no chat open
}

func newFocusRegistry() *focusRegistry {
	return &focusRegistry{on: make(map[clientID]int64)}
}

// attach registers a client with no chat open and returns its id.
func (r *focusRegistry) attach() clientID {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.on[r.next] = 0
	return r.next
}

// set records the chat a client is showing; 0 means it has none open. A set from
// a client that has already detached is dropped: its focus is gone and must not
// come back.
func (r *focusRegistry) set(id clientID, chatID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.on[id]; !ok {
		return
	}
	r.on[id] = chatID
}

// detach forgets a client and the focus it held.
func (r *focusRegistry) detach(id clientID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.on, id)
}

// focused reports that some attached client is looking at chatID. Chat 0 is
// never focused: it is how a client says it has no chat open, not a chat.
func (r *focusRegistry) focused(chatID int64) bool {
	if chatID == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, on := range r.on {
		if on == chatID {
			return true
		}
	}
	return false
}

// Attachment is one client's handle on the owner. It carries the whole owner
// command surface plus the client's own lifecycle: the focus it reports and the
// detach that ends it. A client cannot report focus on another client's behalf,
// because it never holds another client's handle.
//
// In v2 the embedded *Owner becomes a proxy over the socket and nothing above
// changes: a client already talks to its own attachment rather than to a shared
// owner (#183).
type Attachment struct {
	*Owner
	id clientID
}

// Attach registers a client. The caller must Detach when it goes away, or the
// owner goes on believing an abandoned chat is still on screen.
func (o *Owner) Attach() *Attachment {
	return &Attachment{Owner: o, id: o.focus.attach()}
}

// SetFocus reports the chat this client is showing, 0 for none.
func (a *Attachment) SetFocus(chatID int64) { a.focus.set(a.id, chatID) }

// Detach ends the attachment and drops the focus it held.
func (a *Attachment) Detach() { a.focus.detach(a.id) }
