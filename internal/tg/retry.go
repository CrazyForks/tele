package tg

import (
	"context"
	"time"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

const maxRetries = 4

// WithRetry calls fn up to maxRetries times with exponential backoff. Only two
// kinds are worth repeating: a rate limit, once its wait has passed, and a
// transient network failure. Everything else — a missing peer, a forbidden
// action, a stale file reference — fails on the first attempt, because a repeat
// cannot change the answer and the delay is paid by the user.
//
// The rate-limit wait is honoured in full and has no upper bound; capping it is
// tracked separately as #201.
func WithRetry(ctx context.Context, fn func() error) error {
	delay := 500 * time.Millisecond
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
