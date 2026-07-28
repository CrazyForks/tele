package project

import (
	"reflect"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

// ChatDeltaKind names what can happen to a chat subscription. Each kind maps
// onto a setter the chat pane already exposes, which is why the set is this
// shape: it was taken from the working UI, not invented for it.
type ChatDeltaKind int

const (
	// ChatReset replaces the window and the header. Emitted first for every
	// subscription, and whenever the window moved rather than grew: a jump to a
	// quoted message, a chat switch, or any header change.
	ChatReset ChatDeltaKind = iota
	ChatOlder
	ChatNewer
	ChatAppend
	ChatUpdate
	ChatRemove
	ChatRead
	ChatDraft
	ChatTyping
)

type ChatDelta struct {
	Kind            ChatDeltaKind
	Contents        ChatContents
	Messages        []domain.Message
	Message         domain.Message
	MsgIDs          []int
	HasOlder        bool
	HasNewer        bool
	ReadInboxMaxID  int
	ReadOutboxMaxID int
	Draft           string
	Typing          string
}

// DiffChat turns a pair of successive contents into deltas. The message window
// is compared as a sequence of ids: a pure extension at either end is an
// Older/Newer delta, an unchanged sequence with changed payloads is an Update
// per message, one appended id at the live end is an Append, missing ids are a
// Remove, and anything else means the window moved and is a Reset.
func DiffChat(prev, next ChatContents) []ChatDelta {
	if prev.ChatID != next.ChatID || headerChanged(prev, next) {
		return []ChatDelta{{Kind: ChatReset, Contents: next}}
	}

	out := diffWindow(prev, next)

	if prev.ReadInboxMaxID != next.ReadInboxMaxID || prev.ReadOutboxMaxID != next.ReadOutboxMaxID {
		out = append(out, ChatDelta{
			Kind:            ChatRead,
			ReadInboxMaxID:  next.ReadInboxMaxID,
			ReadOutboxMaxID: next.ReadOutboxMaxID,
		})
	}
	if prev.Draft != next.Draft {
		out = append(out, ChatDelta{Kind: ChatDraft, Draft: next.Draft})
	}
	if prev.Typing != next.Typing {
		out = append(out, ChatDelta{Kind: ChatTyping, Typing: next.Typing})
	}
	return out
}

// headerChanged reports a change to the per-chat state rendered around the
// window. It has no delta of its own: it changes rarely, and a Reset is cheaper
// than another kind to get wrong.
func headerChanged(prev, next ChatContents) bool {
	return prev.Title != next.Title || prev.IsUser != next.IsUser || prev.Online != next.Online
}

func diffWindow(prev, next ChatContents) []ChatDelta {
	prevIDs, nextIDs := msgIDs(prev.Messages), msgIDs(next.Messages)

	switch {
	case equalInts(prevIDs, nextIDs):
		if prev.AnchorMsgID != next.AnchorMsgID {
			return []ChatDelta{{Kind: ChatReset, Contents: next}}
		}
		var out []ChatDelta
		for i, m := range next.Messages {
			if !sameMessage(prev.Messages[i], m) {
				out = append(out, ChatDelta{Kind: ChatUpdate, Message: m})
			}
		}
		// The window did not move, but the history either side of it did.
		if prev.HasOlder != next.HasOlder {
			out = append(out, ChatDelta{Kind: ChatOlder, HasOlder: next.HasOlder})
		}
		if prev.HasNewer != next.HasNewer {
			out = append(out, ChatDelta{Kind: ChatNewer, HasNewer: next.HasNewer})
		}
		return out

	case len(prevIDs) == 0:
		return []ChatDelta{{Kind: ChatReset, Contents: next}}

	case hasSuffix(nextIDs, prevIDs): // messages entered above the window
		n := len(nextIDs) - len(prevIDs)
		return []ChatDelta{{Kind: ChatOlder, Messages: next.Messages[:n], HasOlder: next.HasOlder}}

	case hasPrefix(nextIDs, prevIDs): // messages entered below the window
		n := len(prevIDs)
		if len(nextIDs)-n == 1 && !next.HasNewer {
			return []ChatDelta{{Kind: ChatAppend, Message: next.Messages[n]}}
		}
		return []ChatDelta{{Kind: ChatNewer, Messages: next.Messages[n:], HasNewer: next.HasNewer}}

	case isSubsequence(nextIDs, prevIDs): // ids disappeared, the rest kept order
		return []ChatDelta{{Kind: ChatRemove, MsgIDs: missing(prevIDs, nextIDs)}}

	default:
		return []ChatDelta{{Kind: ChatReset, Contents: next}}
	}
}

// sameMessage compares whole messages rather than a hand-picked field list: a
// list rots silently when a field is added, and this diffs at most a window's
// worth of messages per change.
func sameMessage(a, b domain.Message) bool { return reflect.DeepEqual(a, b) }

func msgIDs(ms []domain.Message) []int {
	out := make([]int, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hasPrefix reports whether s starts with p.
func hasPrefix(s, p []int) bool {
	return len(s) >= len(p) && equalInts(s[:len(p)], p)
}

// hasSuffix reports whether s ends with p.
func hasSuffix(s, p []int) bool {
	return len(s) >= len(p) && equalInts(s[len(s)-len(p):], p)
}

// isSubsequence reports whether sub appears in s in order, not necessarily
// contiguously.
func isSubsequence(sub, s []int) bool {
	i := 0
	for _, v := range s {
		if i < len(sub) && sub[i] == v {
			i++
		}
	}
	return i == len(sub)
}

// missing returns the entries of a that do not appear in b.
func missing(a, b []int) []int {
	have := make(map[int]struct{}, len(b))
	for _, v := range b {
		have[v] = struct{}{}
	}
	out := make([]int, 0)
	for _, v := range a {
		if _, ok := have[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}
