package core

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/core/outbox"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// sentGracePeriod bounds how long a successfully sent entry waits for its
// message to arrive through the update path before the worker removes it
// anyway. The send happened; a bubble stuck at "sending" would be a lie.
// A variable so tests can shorten it.
var sentGracePeriod = 5 * time.Second

// idleInterval is how long the worker sleeps when nothing is due and nothing is
// scheduled. It exists only so a missed wake cannot strand the queue forever.
const idleInterval = time.Minute

// RunOutbox drains the durable queue. One goroutine, one request in flight:
// Telegram's rate limits are account-global, so parallel sends across chats buy
// nothing and provoke exactly the FLOOD_WAIT they would be trying to outrun.
// Head-of-line blocking is avoided in the selection instead (outbox.Next).
func (o *Owner) RunOutbox(ctx context.Context) {
	if o.outbox == nil {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		entry, ok := outbox.Next(o.outbox.All(), time.Now())
		if !ok {
			if !o.waitForOutboxWork(ctx) {
				return
			}
			continue
		}
		o.attempt(ctx, entry)
	}
}

// waitForOutboxWork sleeps until something could become due, or a submission
// wakes it. It returns false when the context ended.
func (o *Owner) waitForOutboxWork(ctx context.Context) bool {
	wait := idleInterval
	if at, ok := outbox.EarliestDue(o.outbox.All(), time.Now()); ok {
		if d := time.Until(at); d < wait {
			wait = d
		}
	}
	if wait < 0 {
		wait = 0
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-o.outboxWake:
		return true
	case <-timer.C:
		return true
	}
}

// attempt sends one entry and records what came back.
func (o *Owner) attempt(ctx context.Context, e domain.OutboxEntry) {
	peer, err := o.peer(e.ChatID)
	if err != nil {
		o.recordFailure(e, err)
		return
	}

	e.State = domain.OutboxSending
	if err := o.outbox.Update(e); err != nil {
		o.log.Error("outbox: could not mark an entry in flight", zap.Error(err))
		return
	}
	o.Refresh()

	o.log.Debug("outbox: sending",
		zap.String("ref", e.Ref), zap.Int64("chat_id", e.ChatID), zap.Int("attempt", e.Attempts))

	msgID, err := o.client.SendMessage(ctx, peer, e.Message.Text, e.Message.ReplyToMsgID, e.Message.Entities, e.RandomID)
	if ctx.Err() != nil {
		// The owner is going away. The row stays in "sending" on purpose: the
		// next process resets it and resends with the same random_id.
		return
	}
	if err != nil {
		o.recordFailure(e, err)
		return
	}
	o.recordSent(e, msgID)
}

// recordFailure applies the queue's policy to a refused send.
func (o *Owner) recordFailure(e domain.OutboxEntry, err error) {
	delay, terminal := outbox.Backoff(err, e.Attempts)
	if outbox.CountsAsAttempt(err) {
		e.Attempts++
	}
	if terminal {
		e.State = domain.OutboxFailed
		e.NextAttemptAt = time.Time{}
		if te, ok := telerr.As(err); ok {
			e.ErrKind, e.ErrDetail = te.Kind, te.Detail
		} else {
			e.ErrKind, e.ErrDetail = telerr.Internal, err.Error()
		}
		o.log.Warn("outbox: send failed for good",
			zap.String("ref", e.Ref), zap.String("kind", string(e.ErrKind)))
	} else {
		e.State = domain.OutboxQueued
		e.NextAttemptAt = time.Now().Add(delay)
		e.ErrKind, e.ErrDetail = "", ""
		o.log.Debug("outbox: retrying later",
			zap.String("ref", e.Ref), zap.Duration("in", delay), zap.Int("attempts", e.Attempts))
	}
	if uerr := o.outbox.Update(e); uerr != nil {
		o.log.Error("outbox: could not record a failure", zap.Error(uerr))
	}
	o.Refresh()
}

// recordSent hands the entry over to the update path.
//
// The row is deliberately not deleted here. The reply to messages.sendMessage
// is fed into the update pipeline (updhook, #198), but applied to state by the
// update loop on another goroutine. Deleting here and refreshing would publish
// one delta carrying neither the pending bubble nor the message: a visible
// flicker on every send. So the entry is marked with the confirmed ID and
// removed by the change listener once the message is actually in the chat.
func (o *Owner) recordSent(e domain.OutboxEntry, msgID int) {
	e.SentMsgID = msgID
	if err := o.outbox.Update(e); err != nil {
		o.log.Error("outbox: could not record a send", zap.Error(err))
	}
	o.log.Debug("outbox: sent", zap.String("ref", e.Ref), zap.Int("msg_id", msgID))
	go o.dropIfUndelivered(e.Ref)
}

// dropIfUndelivered removes a sent entry the update path never claimed. Without
// it a message that went out but produced no update would sit at "sending"
// forever, which is a worse lie than dropping the entry: the send did happen.
func (o *Owner) dropIfUndelivered(ref string) {
	time.Sleep(sentGracePeriod)
	cur, ok := o.outbox.Get(ref)
	if !ok || cur.SentMsgID == 0 {
		return
	}
	o.log.Warn("outbox: no update for a sent message, dropping the entry",
		zap.String("ref", ref), zap.Int("msg_id", cur.SentMsgID))
	if err := o.outbox.Delete(ref); err != nil {
		o.log.Error("outbox: could not drop a sent entry", zap.Error(err))
	}
	o.Refresh()
}

// clearSentOutbox removes entries whose message has arrived. Called from the
// change listener, so the pending bubble and the real message swap inside one
// delta rather than across two.
func (o *Owner) clearSentOutbox(chatID int64) {
	if o.outbox == nil {
		return
	}
	for _, e := range o.outbox.ForChat(chatID) {
		if e.SentMsgID == 0 {
			continue
		}
		if _, err := o.messageByID(chatID, e.SentMsgID); err != nil {
			continue
		}
		if err := o.outbox.Delete(e.Ref); err != nil {
			o.log.Error("outbox: could not remove a delivered entry", zap.Error(err))
		}
	}
}
