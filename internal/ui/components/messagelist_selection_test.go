package components_test

import (
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageList_SelectedMessageText_ReturnsFocusedText(t *testing.T) {
	ml := components.NewMessageList(20, 40)
	ml.SetMessages(makeMessages(5))
	text, ok := ml.SelectedMessageText()
	assert.True(t, ok)
	assert.Equal(t, "msg 5", text)
}

func TestMessageList_SelectedMessageText_FollowsCursor(t *testing.T) {
	ml := components.NewMessageList(20, 40)
	ml.SetMessages(makeMessages(5))
	ml.CursorUp()
	ml.CursorUp() // cursor on msg 3
	text, ok := ml.SelectedMessageText()
	assert.True(t, ok)
	assert.Equal(t, "msg 3", text)
}

func TestMessageList_SelectedMessageText_EmptyOnMediaOnly(t *testing.T) {
	ml := components.NewMessageList(20, 40)
	ml.SetMessages([]domain.Message{{ID: 1, ChatID: 1, Photo: &domain.PhotoRef{ID: 42}}})
	text, ok := ml.SelectedMessageText()
	assert.False(t, ok)
	assert.Equal(t, "", text)
}

func TestSelectedGroupMedia(t *testing.T) {
	d, _ := time.Parse(time.RFC3339, "2026-07-24T10:00:00Z")
	ml := components.NewMessageList(24, 60)
	ml.SetMessages([]domain.Message{
		{ID: 1, SenderID: 7, GroupedID: 100, Photo: &domain.PhotoRef{ID: 11}, Date: d},
		{ID: 2, SenderID: 7, GroupedID: 100, Media: &domain.MediaRef{Kind: domain.MediaVideo}, Document: &domain.DocumentRef{ID: 22}, Date: d},
	})
	got := ml.SelectedGroupMedia()
	require.Len(t, got, 2)
	assert.Equal(t, 1, got[0].Index)
	require.NotNil(t, got[0].Photo)
	assert.Equal(t, int64(11), got[0].Photo.ID)
	assert.Equal(t, 2, got[1].Index)
	require.NotNil(t, got[1].Doc)
	assert.Equal(t, int64(22), got[1].Doc.ID)
}

func TestSelection_AnOutboxEntryReportsNoMessageID(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetMessages([]domain.Message{{ID: 10, ChatID: 1, Text: "old"}})
	// SetMessages parks the cursor on the newest item; the entry then becomes it.
	ml.SetOutbox([]domain.OutboxEntry{{
		Ref: "r1", ChatID: 1, State: domain.OutboxFailed,
		Message: &domain.OutboxMessage{Text: "unsent"},
	}})
	ml.CursorDown()

	assert.Equal(t, "r1", ml.SelectedOutboxRef())
	assert.Zero(t, ml.SelectedMessageID(),
		"message actions must no-op on an entry rather than hit the wrong target")
}

func TestSelection_AMessageReportsNoOutboxRef(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetMessages([]domain.Message{{ID: 10, ChatID: 1, Text: "old"}})

	assert.Equal(t, 10, ml.SelectedMessageID())
	assert.Empty(t, ml.SelectedOutboxRef())
}

func TestSelection_CursorUpFromAnEntryReturnsToTheMessages(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetMessages([]domain.Message{{ID: 10, ChatID: 1, Text: "old"}})
	ml.SetOutbox([]domain.OutboxEntry{{
		Ref: "r1", ChatID: 1, State: domain.OutboxFailed,
		Message: &domain.OutboxMessage{Text: "unsent"},
	}})
	ml.CursorDown()
	require.Equal(t, "r1", ml.SelectedOutboxRef())

	ml.CursorUp()

	assert.Equal(t, 10, ml.SelectedMessageID())
	assert.Empty(t, ml.SelectedOutboxRef(), "the two halves of the cursor are mutually exclusive")
}

func TestSelection_SelectedOutboxEntryCarriesItsState(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetOutbox([]domain.OutboxEntry{{
		Ref: "r1", ChatID: 1, State: domain.OutboxFailed, ErrKind: telerr.Forbidden,
		Message: &domain.OutboxMessage{Text: "unsent"},
	}})

	got, ok := ml.SelectedOutboxEntry()

	require.True(t, ok)
	assert.Equal(t, domain.OutboxFailed, got.State)
	assert.Equal(t, telerr.Forbidden, got.ErrKind)
}

// The selection indicator has to appear beside a queued send too: the cursor
// can rest there, and a cursor you cannot see is a cursor you cannot use (#193).
func TestSelection_TheIndicatorIsDrawnOnTheSelectedEntry(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetShowIndicator(true)
	ml.SetMessages([]domain.Message{{ID: 10, ChatID: 1, Text: "old"}})
	ml.SetOutbox([]domain.OutboxEntry{{
		Ref: "r1", ChatID: 1, State: domain.OutboxFailed,
		Message: &domain.OutboxMessage{Text: "unsent"},
	}})
	require.Equal(t, "r1", ml.SelectedOutboxRef())
	onEntry := ml.View()

	ml.CursorUp()
	require.Equal(t, 10, ml.SelectedMessageID())
	onMessage := ml.View()

	assert.NotEqual(t, onEntry, onMessage,
		"moving the cursor off the entry must change what is drawn")
}
