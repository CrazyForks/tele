package project

import "sync"

// Registry holds the live subscriptions and the contents each was last told
// about. It is the only stateful piece of this package: builders and diffs are
// pure, so everything that can go wrong concentrates here.
type Registry struct {
	mu     sync.Mutex
	reader Reader
	next   SubID
	subs   map[SubID]*sub

	// typing holds the ephemeral typing label per chat. It has no persisted
	// state to rebuild from, so the registry carries it between refreshes.
	typing map[int64]string
}

type sub struct {
	window Window
	list   ChatListContents
	chat   ChatContents
}

func NewRegistry(r Reader) *Registry {
	return &Registry{
		reader: r,
		subs:   make(map[SubID]*sub),
		typing: make(map[int64]string),
	}
}

// Subscribe registers a window and replies with its current contents, so a
// resubscribe is a full resync and no snapshot concept is needed.
func (g *Registry) Subscribe(w Window) (SubID, []Delta) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	id := g.next
	g.subs[id] = &sub{window: w}
	return id, g.rebuild(id)
}

// MoveWindow replaces a subscription's window. It carries a whole window rather
// than a delta, so repeating it is equivalent to resubscribing.
func (g *Registry) MoveWindow(id SubID, w Window) []Delta {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.subs[id]
	if !ok {
		return nil
	}
	s.window = w
	return g.rebuild(id)
}

func (g *Registry) Unsubscribe(id SubID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.subs, id)
}

// Window reports a subscription's current window, so the owner can decide
// whether it needs backfilling from Telegram.
func (g *Registry) Window(id SubID) (Window, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.subs[id]
	if !ok {
		return nil, false
	}
	return s.window, true
}

// Refresh rebuilds every subscription and returns whatever actually differs.
// The owner calls it once per applied state change; a change no window contains
// produces no deltas, which is the point.
func (g *Registry) Refresh() []Delta {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rebuildAll()
}

// SetTyping records an ephemeral typing label and refreshes. The label is not
// persisted anywhere, so it cannot be rebuilt from the reader.
func (g *Registry) SetTyping(chatID int64, label string) []Delta {
	g.mu.Lock()
	defer g.mu.Unlock()
	if label == "" {
		delete(g.typing, chatID)
	} else {
		g.typing[chatID] = label
	}
	return g.rebuildAll()
}

// rebuildAll refreshes every subscription. Caller holds the lock. Map iteration
// makes the order across subscriptions unspecified, which is fine: every delta
// is addressed to one subscription.
func (g *Registry) rebuildAll() []Delta {
	var out []Delta
	for id := range g.subs {
		out = append(out, g.rebuild(id)...)
	}
	return out
}

// rebuild recomputes one subscription's contents and diffs it against what the
// client was last told. Caller holds the lock.
func (g *Registry) rebuild(id SubID) []Delta {
	s := g.subs[id]
	var out []Delta
	switch w := s.window.(type) {
	case ChatListWindow:
		next := BuildChatList(g.reader, w)
		for _, d := range DiffChatList(s.list, next) {
			out = append(out, Delta{Sub: id, ChatList: &d})
		}
		s.list = next
	case ChatWindow:
		next := BuildChat(g.reader, w)
		next.Typing = g.typing[w.ChatID]
		for _, d := range DiffChat(s.chat, next) {
			out = append(out, Delta{Sub: id, Chat: &d})
		}
		s.chat = next
	}
	return out
}
