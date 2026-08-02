package outbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

func entry(seq int64, chat int64, state domain.OutboxState, due time.Time) domain.OutboxEntry {
	return domain.OutboxEntry{
		Seq: seq, ChatID: chat, State: state, NextAttemptAt: due,
		Ref: string(rune('a' + seq)), Kind: domain.OutboxText,
	}
}

func TestNext_PicksTheOldestDueEntry(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(2, 20, domain.OutboxQueued, time.Time{}),
		entry(1, 10, domain.OutboxQueued, time.Time{}),
	}

	got, ok := Next(entries, now)

	require.True(t, ok)
	assert.Equal(t, int64(1), got.Seq)
}

func TestNext_SkipsAnEntryStillInBackoff(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(1, 10, domain.OutboxQueued, now.Add(time.Minute)),
		entry(2, 20, domain.OutboxQueued, time.Time{}),
	}

	got, ok := Next(entries, now)

	require.True(t, ok)
	assert.Equal(t, int64(2), got.Seq, "a chat in backoff must not delay a different chat")
}

func TestNext_KeepsFIFOWithinOneChat(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(1, 10, domain.OutboxQueued, now.Add(time.Minute)),
		entry(2, 10, domain.OutboxQueued, time.Time{}),
	}

	_, ok := Next(entries, now)

	assert.False(t, ok, "the second message of a chat must not overtake the first")
}

func TestNext_AFailedHeadBlocksItsOwnChatOnly(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(1, 10, domain.OutboxFailed, time.Time{}),
		entry(2, 10, domain.OutboxQueued, time.Time{}),
		entry(3, 20, domain.OutboxQueued, time.Time{}),
	}

	got, ok := Next(entries, now)

	require.True(t, ok)
	assert.Equal(t, int64(3), got.Seq, "a failed head blocks its chat, and nothing else")
}

func TestNext_IgnoresEntriesAlreadyInFlight(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{entry(1, 10, domain.OutboxSending, time.Time{})}

	_, ok := Next(entries, now)

	assert.False(t, ok)
}

func TestNext_NothingQueuedIsNotAnEntry(t *testing.T) {
	_, ok := Next(nil, time.Unix(1000, 0))

	assert.False(t, ok)
}

func TestEarliestDue_ReportsTheNearestWaitingHead(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(1, 10, domain.OutboxQueued, now.Add(5*time.Minute)),
		entry(2, 20, domain.OutboxQueued, now.Add(time.Minute)),
	}

	at, ok := EarliestDue(entries, now)

	require.True(t, ok)
	assert.Equal(t, now.Add(time.Minute), at)
}

func TestEarliestDue_SaysNothingWhenSomethingIsDueNow(t *testing.T) {
	now := time.Unix(1000, 0)
	entries := []domain.OutboxEntry{
		entry(1, 10, domain.OutboxQueued, now.Add(5*time.Minute)),
		entry(2, 20, domain.OutboxQueued, time.Time{}),
	}

	_, ok := EarliestDue(entries, now)

	assert.False(t, ok, "there is nothing to wait for when Next will hand something over")
}

func TestBackoff_RateLimitWaitsTheServerInterval(t *testing.T) {
	err := &telerr.Error{Kind: telerr.RateLimited, RetryAfter: 12 * time.Minute}

	delay, terminal := Backoff(err, 3)

	assert.False(t, terminal)
	assert.Equal(t, 12*time.Minute, delay)
}

func TestBackoff_TransientNetworkGrowsAndIsCapped(t *testing.T) {
	err := &telerr.Error{Kind: telerr.Network, Transient: true}

	first, terminal := Backoff(err, 0)
	require.False(t, terminal)
	assert.Equal(t, 2*time.Second, first)

	second, _ := Backoff(err, 1)
	assert.Equal(t, 4*time.Second, second)

	capped, _ := Backoff(err, 40)
	assert.Equal(t, 5*time.Minute, capped)
}

func TestBackoff_NonTransientNetworkIsTerminal(t *testing.T) {
	_, terminal := Backoff(&telerr.Error{Kind: telerr.Network}, 0)

	assert.True(t, terminal)
}

func TestBackoff_UnauthorizedWaitsForLogin(t *testing.T) {
	delay, terminal := Backoff(&telerr.Error{Kind: telerr.Unauthorized}, 7)

	assert.False(t, terminal, "the message waits for a login, it is not the message's fault")
	assert.Equal(t, 5*time.Minute, delay)
}

func TestBackoff_TerminalKinds(t *testing.T) {
	for _, kind := range []telerr.Kind{
		telerr.PeerNotFound, telerr.Forbidden, telerr.NotFound, telerr.Internal,
	} {
		_, terminal := Backoff(&telerr.Error{Kind: kind}, 0)
		assert.True(t, terminal, "kind %s must be terminal", kind)
	}
}

func TestBackoff_AnUnmappedErrorIsTerminal(t *testing.T) {
	_, terminal := Backoff(assert.AnError, 0)

	assert.True(t, terminal)
}

func TestCountsAsAttempt_ARateLimitDoesNotAdvanceTheCurve(t *testing.T) {
	assert.False(t, CountsAsAttempt(&telerr.Error{Kind: telerr.RateLimited}))
	assert.True(t, CountsAsAttempt(&telerr.Error{Kind: telerr.Network, Transient: true}))
}
