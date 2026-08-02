package domain

import (
	"time"

	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// OutboxState is where a queued send has got to. There are three, not the four
// a naive reading suggests: "sent" is not a state but the moment the entry is
// deleted, because the real message is in the chat by then and a surviving
// entry would render a second bubble.
type OutboxState string

const (
	// OutboxQueued is waiting for its turn, or for NextAttemptAt to arrive.
	OutboxQueued OutboxState = "queued"
	// OutboxSending has a request in flight. It is reset to OutboxQueued on
	// startup: a process that died mid-send left it behind.
	OutboxSending OutboxState = "sending"
	// OutboxFailed is terminal until the user retries or discards it. Reached
	// only through a non-retryable error kind, never through an attempt count.
	OutboxFailed OutboxState = "failed"
)

// OutboxKind tags the payload variant. #195 adds media here.
type OutboxKind string

// OutboxText is a plain text message, the only variant this release sends
// through the queue.
const OutboxText OutboxKind = "text"

// OutboxMessage is the payload of an OutboxText entry.
type OutboxMessage struct {
	Text         string          `json:"text"`
	Entities     []MessageEntity `json:"entities,omitempty"`
	ReplyToMsgID int             `json:"reply_to,omitempty"`
}

// OutboxEntry is one durable send. It crosses the owner boundary inside the
// chat projection and is JSON on the wire in v2, which is why it lives in
// domain rather than beside the queue that stores it.
type OutboxEntry struct {
	// Ref is the client's idempotency key, and the seed RandomID is derived
	// from. Resubmitting a known Ref is a no-op.
	Ref string
	// Seq is the submission order (the table's rowid). FIFO within a chat is
	// ordering by this.
	Seq    int64
	ChatID int64
	// RandomID is Telegram's deduplication key, derived from Ref so that a
	// resubmission in the window before anything was persisted still produces
	// the same value.
	RandomID int64
	Kind     OutboxKind
	State    OutboxState
	// Message is set when Kind is OutboxText.
	Message *OutboxMessage
	// Attempts counts failures that led to a backoff. It drives the backoff
	// curve and the display, and never decides terminality.
	Attempts      int
	CreatedAt     time.Time
	NextAttemptAt time.Time
	// SentMsgID is the ID Telegram confirmed. Set between a successful request
	// and the moment the message lands in state, which is when the entry goes.
	// It is deliberately not persisted: a crash in that window must re-send
	// rather than assume, and the persisted RandomID makes that safe.
	SentMsgID int
	// ErrKind and ErrDetail are empty unless State is OutboxFailed.
	ErrKind   telerr.Kind
	ErrDetail string
}
