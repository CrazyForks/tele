package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
	"github.com/sorokin-vladimir/tele/internal/ui"
)

// rootOnFailedSend opens a chat holding one failed queued send, with the cursor
// on it.
func rootOnFailedSend(t *testing.T) (ui.RootModel, *testOwner) {
	t.Helper()
	m, _ := newRootWithOpenChat(t, &mockTGClient{})
	m = m.WithFocus(ui.FocusChat)
	m.Chat().SetOutbox([]domain.OutboxEntry{{
		Ref: "r1", ChatID: 1, State: domain.OutboxFailed, ErrKind: telerr.Forbidden,
		Message: &domain.OutboxMessage{Text: "unsent"},
	}})
	require.Equal(t, "r1", m.Chat().SelectedOutboxRef())
	return m, m.Owner().(*testOwner)
}

func TestEnter_RetriesTheSelectedFailedSend(t *testing.T) {
	m, owner := rootOnFailedSend(t)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()

	assert.Equal(t, []string{"r1"}, owner.retried)
}

func TestEnter_DoesNothingOnAMessage(t *testing.T) {
	m, _ := newRootWithOpenChat(t, &mockTGClient{})
	m = m.WithFocus(ui.FocusChat)
	owner := m.Owner().(*testOwner)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	assert.Empty(t, owner.retried)
}

func TestContextMenu_OnAnEntryOffersRetryAndDiscard(t *testing.T) {
	m, owner := rootOnFailedSend(t)

	// space opens the menu, d discards.
	next, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = next.(ui.RootModel)
	require.True(t, m.ContextMenuOpen(), "an entry must open a menu of its own")

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = next.(ui.RootModel)
	require.NotNil(t, cmd)
	next, cmd2 := m.Update(cmd())
	m = next.(ui.RootModel)
	require.NotNil(t, cmd2)
	cmd2()

	assert.Equal(t, []string{"r1"}, owner.discarded)
	assert.False(t, m.ContextMenuOpen(), "the menu closes after the action")
}

// x is composer state only. A staged reply and a selected failed send are
// different targets, and x must keep aiming at the first (#193).
func TestCancelUpload_ClearsTheReplyAndLeavesTheQueueAlone(t *testing.T) {
	m, owner := rootOnFailedSend(t)
	m.Chat().SetReply(10, "quoted", "Ada")
	require.Equal(t, 10, m.Chat().ReplyToMsgID())

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = next.(ui.RootModel)

	assert.Zero(t, m.Chat().ReplyToMsgID(), "x clears the reply")
	assert.Len(t, m.Chat().Outbox(), 1, "and leaves the queue alone")
	assert.Empty(t, owner.discarded)
}
