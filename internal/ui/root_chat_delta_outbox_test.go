package ui

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

func queuedEntry(ref, text string) domain.OutboxEntry {
	return domain.OutboxEntry{
		Ref: ref, ChatID: 1, State: domain.OutboxQueued,
		Message: &domain.OutboxMessage{Text: text},
	}
}

// A reset carries the whole contents, and the queue is part of it: a chat
// reopened with a send still pending must show it (#193).
func TestChatReset_SeatsTheOutbox(t *testing.T) {
	_, m := anchorTestModel()

	m, _ = m.handleChatDelta(&project.ChatDelta{
		Kind: project.ChatReset,
		Contents: project.ChatContents{
			ChatID:   1,
			Messages: mediaWindow(),
			Outbox:   []domain.OutboxEntry{queuedEntry("r1", "pending")},
		},
	})

	got := m.chat.Outbox()
	if len(got) != 1 || got[0].Ref != "r1" {
		t.Fatalf("the reset must seat the queue, got %+v", got)
	}
}

func TestChatOutboxDelta_ReplacesTheQueue(t *testing.T) {
	_, m := anchorTestModel()
	m, _ = m.handleChatDelta(&project.ChatDelta{
		Kind:   project.ChatOutbox,
		Outbox: []domain.OutboxEntry{queuedEntry("r1", "pending")},
	})

	// The send landed: the owner dropped the entry and published the new list.
	m, _ = m.handleChatDelta(&project.ChatDelta{Kind: project.ChatOutbox})

	if got := m.chat.Outbox(); len(got) != 0 {
		t.Fatalf("a delivered entry must leave the pane, got %+v", got)
	}
}
