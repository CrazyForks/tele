package core

import (
	"context"

	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

// MergeOlder merges an older history chunk in front of the messages already
// held, dropping chunk entries whose IDs are already present. Duplicate
// in-flight loads or overlapping server pages would otherwise seed duplicates
// that render as a repeating date range (issue #120).
//
// Moved here from internal/ui: a client no longer knows whether a window is
// filled from disk or from the network, so the dedup belongs on this side.
func MergeOlder(older, existing []domain.Message) []domain.Message {
	if len(existing) == 0 {
		return older
	}
	seen := make(map[int]struct{}, len(existing))
	for _, m := range existing {
		seen[m.ID] = struct{}{}
	}
	combined := make([]domain.Message, 0, len(older)+len(existing))
	for _, m := range older {
		if _, dup := seen[m.ID]; dup {
			continue
		}
		combined = append(combined, m)
	}
	return append(combined, existing...)
}

// backfill fetches older history for a chat subscription whose window the store
// could not fill and applies it to state; the registry then emits the resulting
// delta through the same path as any other change. One fetch per subscription is
// in flight at a time (issue #120).
func (o *Owner) backfill(ctx context.Context, id project.SubID, w project.ChatWindow) {
	if !o.beginFetch(id) {
		return
	}
	defer o.endFetch(id)

	chat, ok := o.state.Store().GetChat(w.ChatID)
	if !ok {
		return
	}
	existing := o.state.Store().Messages(w.ChatID)
	// Page backwards from the oldest message held; zero means "from the newest",
	// which is what an empty window needs.
	offsetID := 0
	if len(existing) > 0 {
		offsetID = existing[0].ID
	}
	fetched, err := o.client.GetHistory(ctx, chat.Peer, offsetID, o.historyLimit)
	if err != nil {
		o.log.Warn("history backfill failed", zap.Int64("chat", w.ChatID), zap.Error(err))
		return
	}
	if len(fetched) == 0 {
		return
	}
	merged := MergeOlder(fetched, existing)
	if len(merged) == len(existing) {
		// Every fetched message was already held: the chat has no more history.
		return
	}
	o.state.ApplyHistory(w.ChatID, merged)
}

func (o *Owner) beginFetch(id project.SubID) bool {
	o.fetchMu.Lock()
	defer o.fetchMu.Unlock()
	if o.fetching[id] {
		return false
	}
	o.fetching[id] = true
	return true
}

func (o *Owner) endFetch(id project.SubID) {
	o.fetchMu.Lock()
	defer o.fetchMu.Unlock()
	delete(o.fetching, id)
}
