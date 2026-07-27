package tg

import (
	"context"
	"time"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

const maxRetries = 4

// rateLimitBudget bounds how long one WithRetry call may sit out rate limits in
// total. A var, not a const, so tests can shrink it: a const would make them
// wait the real 45 seconds.
//
// The limit is a budget for the whole call rather than a cap per wait because a
// rate-limited attempt continues the loop, so a single call can otherwise
// perform maxRetries+1 waits in a row and a per-wait cap would bound none of it
// (#201).
var rateLimitBudget = 45 * time.Second

// WithRetry calls fn up to maxRetries times with exponential backoff. Only two
// kinds are worth repeating: a rate limit, while the budget lasts, and a
// transient network failure. Everything else — a missing peer, a forbidden
// action, a stale file reference — fails on the first attempt, because a repeat
// cannot change the answer and the delay is paid by the user.
//
// A rate limit too long to sit out is returned to the caller carrying
// Telegram's full RetryAfter, so the UI can say how long the wait really is
// instead of appearing frozen for it.
func WithRetry(ctx context.Context, fn func() error) error {
	delay := 500 * time.Millisecond
	budget := rateLimitBudget
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}

		e, ok := telerr.As(err)
		if !ok {
			return err
		}
		switch e.Kind {
		case telerr.RateLimited:
			if e.RetryAfter > budget {
				return err
			}
			budget -= e.RetryAfter
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(e.RetryAfter):
			}
			continue
		case telerr.Network:
			if !e.Transient {
				return err
			}
		default:
			return err
		}

		if attempt == maxRetries {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
	return nil
}
