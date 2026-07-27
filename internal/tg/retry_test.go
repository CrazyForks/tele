package tg_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/telerr"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
)

func TestWithRetry_SuccessOnFirstCall(t *testing.T) {
	calls := 0
	err := internaltg.WithRetry(context.Background(), func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_RetriesOnTransientError(t *testing.T) {
	calls := 0
	err := internaltg.WithRetry(context.Background(), func() error {
		calls++
		if calls < 3 {
			return &telerr.Error{Kind: telerr.Network, Transient: true}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestWithRetry_GivesUpAfterMaxRetries(t *testing.T) {
	calls := 0
	err := internaltg.WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.Network, Transient: true}
	})
	assert.Error(t, err)
	assert.Equal(t, 5, calls)
}

func TestWithRetry_NonRetryableKindFailsFast(t *testing.T) {
	calls := 0
	err := internaltg.WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.PeerNotFound, Op: "messages.getHistory"}
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "a non-retryable kind must not be retried")
	assert.Equal(t, telerr.PeerNotFound, telerr.Of(err))
}

func TestWithRetry_StaleReferenceFailsFast(t *testing.T) {
	calls := 0
	_ = internaltg.WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.StaleReference}
	})
	assert.Equal(t, 1, calls, "retrying without refreshing the reference cannot help")
}

func TestWithRetry_PlainErrorFailsFast(t *testing.T) {
	calls := 0
	_ = internaltg.WithRetry(context.Background(), func() error {
		calls++
		return errors.New("disk full")
	})
	assert.Equal(t, 1, calls)
}

func TestWithRetry_NonTransientNetworkFailsFast(t *testing.T) {
	calls := 0
	_ = internaltg.WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.Network, Transient: false}
	})
	assert.Equal(t, 1, calls)
}

func TestWithRetry_RateLimitedWaitsAndRetries(t *testing.T) {
	calls := 0
	start := time.Now()
	err := internaltg.WithRetry(context.Background(), func() error {
		calls++
		if calls == 1 {
			return &telerr.Error{Kind: telerr.RateLimited, RetryAfter: 20 * time.Millisecond}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
}

func TestWithRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := internaltg.WithRetry(ctx, func() error {
		return errors.New("fail")
	})
	assert.ErrorIs(t, err, context.Canceled)
}
