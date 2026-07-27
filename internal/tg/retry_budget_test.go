package tg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// withBudget shrinks the rate-limit budget for one test. Lives in an internal
// test file because the budget is unexported; the alternative would be a test
// running for the real 45 seconds.
func withBudget(t *testing.T, d time.Duration) {
	t.Helper()
	prev := rateLimitBudget
	rateLimitBudget = d
	t.Cleanup(func() { rateLimitBudget = prev })
}

func TestWithRetry_RateLimitInsideBudgetIsSatOut(t *testing.T) {
	withBudget(t, 200*time.Millisecond)

	calls := 0
	start := time.Now()
	err := WithRetry(context.Background(), func() error {
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

func TestWithRetry_RateLimitOverBudgetReturnsAtOnce(t *testing.T) {
	withBudget(t, 50*time.Millisecond)

	calls := 0
	start := time.Now()
	err := WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.RateLimited, RetryAfter: 10 * time.Minute}
	})

	require.Error(t, err)
	assert.Equal(t, 1, calls, "a wait beyond the budget must not be sat out")
	assert.Less(t, time.Since(start), time.Second, "the call must return without waiting")

	e, ok := telerr.As(err)
	require.True(t, ok)
	// The user needs the real wait, not what was left of our budget.
	assert.Equal(t, 10*time.Minute, e.RetryAfter)
}

// The case a per-wait cap would get wrong, and the reason the limit is a budget:
// each wait fits on its own, the two together do not.
func TestWithRetry_WaitsFittingAloneButNotTogether(t *testing.T) {
	withBudget(t, 50*time.Millisecond)

	calls := 0
	err := WithRetry(context.Background(), func() error {
		calls++
		return &telerr.Error{Kind: telerr.RateLimited, RetryAfter: 30 * time.Millisecond}
	})

	require.Error(t, err)
	assert.Equal(t, 2, calls, "first wait is sat out, the second exceeds what is left")
}
