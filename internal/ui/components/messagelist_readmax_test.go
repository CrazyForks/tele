package components_test

import (
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

// An album is one visible item but several Telegram messages. Reading it must
// cover every part, otherwise the non-anchor parts stay unread forever.
func TestMessageList_VisibleReadMaxID_CoversWholeAlbum(t *testing.T) {
	now := time.Now()
	msgs := []domain.Message{
		{ID: 10, ChatID: 1, SenderID: 7, Text: "before", Date: now},
		{ID: 11, ChatID: 1, SenderID: 7, GroupedID: 555, Text: "caption", Date: now},
		{ID: 12, ChatID: 1, SenderID: 7, GroupedID: 555, Date: now},
		{ID: 13, ChatID: 1, SenderID: 7, GroupedID: 555, Date: now},
		{ID: 14, ChatID: 1, SenderID: 7, GroupedID: 555, Date: now},
	}

	ml := components.NewMessageList(60, 60)
	ml.SetMessages(msgs)

	assert.Equal(t, 14, ml.VisibleReadMaxID())
}

// An album that is scrolled out of the viewport stays unread, parts included.
func TestMessageList_VisibleReadMaxID_IgnoresAlbumBelowViewport(t *testing.T) {
	now := time.Now()
	msgs := []domain.Message{
		{ID: 10, ChatID: 1, SenderID: 7, Text: "before", Date: now},
		{ID: 11, ChatID: 1, SenderID: 7, GroupedID: 555, Text: "caption", Date: now},
		{ID: 12, ChatID: 1, SenderID: 7, GroupedID: 555, Date: now},
	}

	ml := components.NewMessageList(60, 60)
	ml.SetMessages(msgs)
	ml.SetSize(60, 3) // only the top of the list fits
	for i := 0; i < 30; i++ {
		ml.ScrollUp()
	}

	assert.Less(t, ml.VisibleReadMaxID(), 11)
}
