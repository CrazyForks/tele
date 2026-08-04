package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// mediaEntry is one queued album group, as the chat projection delivers it.
func mediaEntry(ref string, state domain.OutboxState, parts ...domain.OutboxMediaPart) domain.OutboxEntry {
	return domain.OutboxEntry{
		Ref: ref, ChatID: 1, Kind: domain.OutboxMedia, State: state,
		Media: &domain.OutboxMediaSend{Parts: parts},
	}
}

func TestOutboxMedia_AGroupIsOneBubbleNamingTheCount(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 10, SendAs: domain.MediaPhoto},
		domain.OutboxMediaPart{Name: "b.jpg", Size: 10, SendAs: domain.MediaPhoto},
	)})

	out := ml.View()

	assert.Contains(t, out, "2 photos", "a group is one bubble, as it will be one album")
	assert.NotContains(t, out, "a.jpg", "individual names belong to a single-file send")
}

func TestOutboxMedia_ALoneFileIsNamed(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "holiday.jpg", Size: 10, SendAs: domain.MediaPhoto},
	)})

	assert.Contains(t, ml.View(), "holiday.jpg")
}

func TestOutboxMedia_CarriesTheCaption(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	e := mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 10, SendAs: domain.MediaPhoto})
	e.Media.Caption = "from the trip"
	ml.SetOutbox([]domain.OutboxEntry{e})

	assert.Contains(t, ml.View(), "from the trip")
}

func TestOutboxMedia_ProgressAdvancesTheBar(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 100, SendAs: domain.MediaPhoto},
		domain.OutboxMediaPart{Name: "b.jpg", Size: 100, SendAs: domain.MediaPhoto},
	)})

	ml.SetUploadProgress("r1#0", 2, 2, 0.75)

	out := ml.View()
	assert.Contains(t, out, "75%")
	assert.Contains(t, out, "2/2", "which file of how many is the useful part of an album's progress")
}

func TestOutboxMedia_ProgressForAnEntryThatLeftIsForgotten(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 100, SendAs: domain.MediaPhoto},
	)})
	ml.SetUploadProgress("r1#0", 1, 1, 0.5)

	// The send landed: the entry goes, and a second one reuses nothing of it.
	ml.SetOutbox(nil)
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxQueued,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 100, SendAs: domain.MediaPhoto},
	)})

	assert.NotContains(t, ml.View(), "50%", "a fresh entry must not inherit a stale bar")
}

func TestOutboxMedia_TheCursorAddressesTheWholeGroup(t *testing.T) {
	ml := components.NewMessageList(20, 60)
	ml.SetMessages([]domain.Message{{ID: 10, ChatID: 1, Text: "old"}})
	ml.SetOutbox([]domain.OutboxEntry{mediaEntry("r1#0", domain.OutboxUploading,
		domain.OutboxMediaPart{Name: "a.jpg", Size: 10, SendAs: domain.MediaPhoto},
		domain.OutboxMediaPart{Name: "b.jpg", Size: 10, SendAs: domain.MediaPhoto},
	)})
	ml.CursorDown()

	assert.Equal(t, "r1#0", ml.SelectedOutboxRef(),
		"discarding is per entry, so the cursor must reach the entry")
	entry, ok := ml.SelectedOutboxEntry()
	require.True(t, ok)
	require.NotNil(t, entry.Media)
	assert.Len(t, entry.Media.Parts, 2)
}
