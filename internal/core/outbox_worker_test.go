package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/sorokin-vladimir/tele/internal/core/outbox"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// waitFor polls until cond holds or the deadline passes. The worker runs on its
// own goroutine, so assertions have to wait for it rather than assume.
func waitFor(t *testing.T, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within the deadline: %s", msg)
}

// runWorker starts the drain loop and stops it when the test ends.
func runWorker(t *testing.T, o *Owner) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go o.RunOutbox(ctx)
	return ctx
}

func TestWorker_SendsAQueuedEntryWithItsPersistedRandomID(t *testing.T) {
	c := &stubClient{sentID: 77}
	o, _ := newCmdOwner(t, c)
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))

	waitFor(t, "the entry was never sent", func() bool { return c.sendCalls() == 1 })
	assert.Equal(t, outbox.RandomIDFor("r1"), c.lastRandomID())
}

func TestWorker_MarksATerminalKindFailedAndKeepsTheText(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Forbidden, Detail: "CHAT_WRITE_FORBIDDEN"}}
	o, _ := newCmdOwner(t, c)
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))

	waitFor(t, "the entry never reached failed", func() bool {
		e, ok := q.Get("r1")
		return ok && e.State == domain.OutboxFailed
	})
	e, _ := q.Get("r1")
	assert.Equal(t, telerr.Forbidden, e.ErrKind)
	assert.Equal(t, "CHAT_WRITE_FORBIDDEN", e.ErrDetail)
	require.NotNil(t, e.Message)
	assert.Equal(t, "hi", e.Message.Text, "a failed send must not lose what the user typed")
}

func TestWorker_BacksOffATransientNetworkFailureWithoutFailing(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Network, Transient: true}}
	o, _ := newCmdOwner(t, c)
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))

	waitFor(t, "the attempt was never recorded", func() bool {
		e, ok := q.Get("r1")
		return ok && e.Attempts > 0
	})
	e, _ := q.Get("r1")
	assert.Equal(t, domain.OutboxQueued, e.State, "an outage is not a reason to give up on the message")
	assert.True(t, e.NextAttemptAt.After(time.Now()))
}

func TestWorker_ARateLimitDoesNotAdvanceTheAttemptCurve(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.RateLimited, RetryAfter: time.Hour}}
	o, _ := newCmdOwner(t, c)
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))

	waitFor(t, "the wait was never scheduled", func() bool {
		e, ok := q.Get("r1")
		return ok && !e.NextAttemptAt.IsZero()
	})
	e, _ := q.Get("r1")
	assert.Zero(t, e.Attempts, "an assigned wait is not a failure of ours")
	assert.Equal(t, domain.OutboxQueued, e.State)
}

func TestWorker_AFailedEntryBlocksItsOwnChatOnly(t *testing.T) {
	c := &stubClient{err: &telerr.Error{Kind: telerr.Forbidden}}
	o, st := newCmdOwner(t, c)
	st.SetChat(domain.Chat{ID: 2, Title: "Bob", Peer: domain.Peer{ID: 2, Type: domain.PeerUser}})
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "blocked"}))
	waitFor(t, "the first entry never failed", func() bool {
		e, ok := q.Get("r1")
		return ok && e.State == domain.OutboxFailed
	})
	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r2", ChatID: 1, Text: "behind it"}))
	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r3", ChatID: 2, Text: "other chat"}))

	waitFor(t, "the other chat never got its turn", func() bool {
		e, ok := q.Get("r3")
		return ok && e.State == domain.OutboxFailed
	})
	behind, ok := q.Get("r2")
	require.True(t, ok)
	assert.Equal(t, domain.OutboxQueued, behind.State,
		"letting it past a failed entry would reorder the conversation")
}

func TestWorker_RestartResendsWithTheSameRandomID(t *testing.T) {
	// The crash test the issue calls the point of the exercise: die between
	// persisting the entry and hearing back, restart, and assert that what goes
	// out the second time carries the identical deduplication key.
	path := filepath.Join(t.TempDir(), "q.db")
	// Two handles on one file is the SQLITE_BUSY hazard of #119, so the first is
	// closed before the second opens — which is also what a real restart does.
	openQueue := func() (*outbox.Store, *sql.DB) {
		db, err := sql.Open("sqlite", path)
		require.NoError(t, err)
		db.SetMaxOpenConns(1)
		s, err := outbox.NewStore(db)
		require.NoError(t, err)
		return s, db
	}

	blocked := make(chan struct{})
	first := &stubClient{sendBlock: blocked}
	o1, _ := newCmdOwner(t, first)
	q1, db1 := openQueue()
	o1.SetOutbox(q1)
	ctx1, cancel1 := context.WithCancel(context.Background())
	go o1.RunOutbox(ctx1)
	require.NoError(t, o1.Send(ctx1, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))
	waitFor(t, "the first attempt never went out", func() bool { return first.sendCalls() == 1 })
	firstRandomID := first.lastRandomID()

	// The process dies mid-flight: the row is left in "sending".
	cancel1()
	close(blocked)
	waitFor(t, "the entry was never marked in flight", func() bool {
		e, ok := q1.Get("r1")
		return ok && e.State == domain.OutboxSending
	})
	require.NoError(t, db1.Close())

	second := &stubClient{sentID: 77}
	o2, _ := newCmdOwner(t, second)
	q2, db2 := openQueue()
	t.Cleanup(func() { _ = db2.Close() })
	o2.SetOutbox(q2)
	runWorker(t, o2)

	waitFor(t, "the restarted owner never resent", func() bool { return second.sendCalls() == 1 })
	assert.Equal(t, firstRandomID, second.lastRandomID(),
		"the resend must reuse the random_id, or Telegram cannot deduplicate it")
}

func TestWorker_TheEntryGoesWhenTheMessageLands(t *testing.T) {
	// The pending bubble and the real message must swap inside one delta. The
	// worker therefore leaves the entry alone on success and the change listener
	// removes it, so no delta carries neither one (#193).
	c := &stubClient{sentID: 77}
	o, _ := newCmdOwner(t, c)
	q := newOutboxStore(t)
	o.SetOutbox(q)
	ctx := runWorker(t, o)

	require.NoError(t, o.Send(ctx, SendRequest{Ref: "r1", ChatID: 1, Text: "hi"}))
	waitFor(t, "the entry was never sent", func() bool { return c.sendCalls() == 1 })

	entry, ok := q.Get("r1")
	require.True(t, ok, "the entry must outlive the request that succeeded")
	assert.Equal(t, 77, entry.SentMsgID)

	// The update path applies the message the reply carried.
	o.state.ApplyIncoming(domain.Message{ID: 77, ChatID: 1, Text: "hi", IsOut: true})

	waitFor(t, "the delivered entry was never removed", func() bool {
		_, still := q.Get("r1")
		return !still
	})
}
