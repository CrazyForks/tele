// Package telerr defines the closed set of error kinds that may leave the
// Telegram layer. Mapping gotd errors onto it happens exactly once, in
// internal/tg; nothing outside that package inspects gotd error values.
//
// This package deliberately imports nothing but the standard library: in v2 it
// is shared by the owner process and by clients that never link gotd at all.
package telerr

import (
	"errors"
	"time"
)

// Kind classifies a failure. The set is closed, so callers switch over it
// exhaustively rather than matching sentinels.
type Kind string

const (
	// Unauthorized means the session is invalid or missing. Needs auth.
	Unauthorized Kind = "unauthorized"
	// RateLimited means Telegram asked us to wait. Carries RetryAfter.
	RateLimited Kind = "rate_limited"
	// PeerNotFound means the peer is unknown or inaccessible.
	PeerNotFound Kind = "peer_not_found"
	// Forbidden means the action is not allowed here: restriction, ban, slowmode.
	Forbidden Kind = "forbidden"
	// NotFound means the message or media is gone.
	NotFound Kind = "not_found"
	// StaleReference means the file reference we hold expired. Re-fetch it and
	// retry; the media itself still exists.
	StaleReference Kind = "stale_reference"
	// Network means a transport failure. Carries Transient.
	Network Kind = "network"
	// Internal means a bug or an unmapped condition.
	Internal Kind = "internal"
)

// Error is the only error shape that leaves internal/tg.
//
// The fields are exported and Kind is a string because in v2 this value is the
// payload of the IPC error frame, with no second representation.
type Error struct {
	Kind Kind `json:"kind"`
	// Op names the failed operation, usually the TL method, e.g.
	// "messages.sendMessage".
	Op string `json:"op,omitempty"`
	// Detail carries the raw Telegram error type for logs, and is the only
	// thing worth showing a user when Kind is Internal.
	Detail string `json:"detail,omitempty"`
	// RetryAfter is set for RateLimited only.
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	// Transient is set for Network only, and reports whether a retry may help.
	Transient bool `json:"transient,omitempty"`
	// Cause is the underlying error. It is not serialisable and never crosses a
	// process boundary, but it must be preserved in-process: gotd recognises
	// its own errors through the unwrap chain.
	Cause error `json:"-"`
}

func (e *Error) Error() string {
	msg := string(e.Kind)
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	if e.Op != "" {
		return e.Op + ": " + msg
	}
	return msg
}

// Unwrap keeps the original error reachable. gotd matches its own errors with
// tgerr.Is, which is errors.As underneath, so breaking this chain breaks the
// library's internal logic silently.
func (e *Error) Unwrap() error { return e.Cause }

// As reports whether err is a domain error and returns it. Use it when the
// kind alone is not enough, e.g. to read RetryAfter.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Of reports the kind of err, walking the wrap chain. A nil error has no kind;
// anything unrecognised is Internal.
func Of(err error) Kind {
	if err == nil {
		return ""
	}
	if e, ok := As(err); ok {
		return e.Kind
	}
	return Internal
}
