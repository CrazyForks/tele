package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

func historyMsgs(ids ...int) []domain.Message {
	out := make([]domain.Message, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.Message{ID: id})
	}
	return out
}

func historyIDs(ms []domain.Message) []int {
	out := make([]int, 0, len(ms))
	for _, x := range ms {
		out = append(out, x.ID)
	}
	return out
}

func TestMergeOlder_PrependsAChunk(t *testing.T) {
	got := core.MergeOlder(historyMsgs(1, 2), historyMsgs(3, 4))

	assert.Equal(t, []int{1, 2, 3, 4}, historyIDs(got))
}

func TestMergeOlder_DropsOverlappingIDs(t *testing.T) {
	// Overlapping server pages would otherwise stack into a repeating date
	// range — issue #120.
	got := core.MergeOlder(historyMsgs(1, 2, 3), historyMsgs(3, 4))

	assert.Equal(t, []int{1, 2, 3, 4}, historyIDs(got))
}

func TestMergeOlder_EmptyExistingReturnsTheChunk(t *testing.T) {
	got := core.MergeOlder(historyMsgs(1, 2), nil)

	assert.Equal(t, []int{1, 2}, historyIDs(got))
}

func TestMergeOlder_FullyDuplicateChunkChangesNothing(t *testing.T) {
	got := core.MergeOlder(historyMsgs(3, 4), historyMsgs(3, 4))

	assert.Equal(t, []int{3, 4}, historyIDs(got))
}
