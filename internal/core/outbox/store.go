package outbox

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

// schema lives here rather than in internal/store because the queue is not part
// of the account cache. It shares the file — and therefore the single
// connection the file DB is limited to (#119) — but nothing else.
//
// AUTOINCREMENT is load-bearing. Without it SQLite reuses the rowid of a
// deleted row, and rows are deleted on success, so submission order would
// silently reorder.
const schema = `
CREATE TABLE IF NOT EXISTS outbox (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	ref             TEXT    NOT NULL UNIQUE,
	chat_id         INTEGER NOT NULL,
	random_id       INTEGER NOT NULL,
	kind            TEXT    NOT NULL,
	payload         TEXT    NOT NULL,
	state           TEXT    NOT NULL,
	attempts        INTEGER NOT NULL DEFAULT 0,
	next_attempt_at INTEGER NOT NULL DEFAULT 0,
	created_at      INTEGER NOT NULL,
	err_kind        TEXT    NOT NULL DEFAULT '',
	err_detail      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_outbox_due ON outbox(next_attempt_at, id);
`

// Store is the durable queue. Reads are served from memory and every write goes
// through to disk, the same shape SQLiteStore uses for chats: the chat
// projection rebuilds on every delta, and going to SQLite for that on the one
// connection it shares with the update path is waste for a table that normally
// holds nothing.
type Store struct {
	mu      sync.RWMutex
	db      *sql.DB
	entries map[string]domain.OutboxEntry
}

// NewStore creates the table if needed, loads what is there, and resets every
// entry left in flight by a process that died mid-send.
func NewStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	s := &Store{db: db, entries: make(map[string]domain.OutboxEntry)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, s.resetSending()
}

// RandomIDFor derives Telegram's deduplication key from the caller's ref.
// Derived rather than generated and stored: a client that resubmits the same
// ref after losing the acknowledgement produces the same value even in the
// window before anything was written down.
func RandomIDFor(ref string) int64 {
	sum := sha256.Sum256([]byte(ref))
	id := int64(binary.LittleEndian.Uint64(sum[:8]))
	if id == 0 {
		return 1
	}
	return id
}

func (s *Store) load() error {
	rows, err := s.db.Query(`SELECT id, ref, chat_id, random_id, kind, payload, state,
		attempts, next_attempt_at, created_at, err_kind, err_detail FROM outbox`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			e                    domain.OutboxEntry
			payload              string
			kind, state, errKind string
			nextAttempt, created int64
		)
		if err := rows.Scan(&e.Seq, &e.Ref, &e.ChatID, &e.RandomID, &kind, &payload,
			&state, &e.Attempts, &nextAttempt, &created, &errKind, &e.ErrDetail); err != nil {
			return err
		}
		e.Kind = domain.OutboxKind(kind)
		e.State = domain.OutboxState(state)
		e.ErrKind = telerr.Kind(errKind)
		if nextAttempt > 0 {
			e.NextAttemptAt = time.UnixMilli(nextAttempt)
		}
		e.CreatedAt = time.UnixMilli(created)
		switch e.Kind {
		case domain.OutboxText:
			var m domain.OutboxMessage
			if err := json.Unmarshal([]byte(payload), &m); err != nil {
				return err
			}
			e.Message = &m
		case domain.OutboxMedia:
			var m domain.OutboxMediaSend
			if err := json.Unmarshal([]byte(payload), &m); err != nil {
				return err
			}
			e.Media = &m
		}
		s.entries[e.Ref] = e
	}
	return rows.Err()
}

// marshalPayload renders the entry's payload column. One column, two shapes,
// chosen by kind: the queue does not care what is in it.
func marshalPayload(e domain.OutboxEntry) ([]byte, error) {
	if e.Kind == domain.OutboxMedia {
		return json.Marshal(e.Media)
	}
	return json.Marshal(e.Message)
}

// resetSending turns every entry a dead process left in flight — uploading its
// bytes or waiting on a request — back into a queued one. The request either
// never reached Telegram or its confirmation never reached us; the persisted
// random_id makes repeating it safe either way, and an upload with nothing
// checkpointed starts again from the top.
func (s *Store) resetSending() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ref, e := range s.entries {
		if e.State != domain.OutboxSending && e.State != domain.OutboxUploading {
			continue
		}
		e.State = domain.OutboxQueued
		e.NextAttemptAt = time.Time{}
		e.SentMsgIDs = nil
		s.entries[ref] = e
	}
	_, err := s.db.Exec(
		`UPDATE outbox SET state = ?, next_attempt_at = 0 WHERE state IN (?, ?)`,
		string(domain.OutboxQueued), string(domain.OutboxSending), string(domain.OutboxUploading))
	return err
}

// Add persists a new entry and returns it with its assigned Seq. added is false
// when the ref is already known, which is what makes submission idempotent.
func (s *Store) Add(e domain.OutboxEntry) (domain.OutboxEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.entries[e.Ref]; ok {
		return existing, false, nil
	}
	payload, err := marshalPayload(e)
	if err != nil {
		return domain.OutboxEntry{}, false, err
	}
	res, err := s.db.Exec(
		`INSERT INTO outbox (ref, chat_id, random_id, kind, payload, state, attempts,
			next_attempt_at, created_at, err_kind, err_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '')`,
		e.Ref, e.ChatID, e.RandomID, string(e.Kind), string(payload), string(e.State),
		e.Attempts, millis(e.NextAttemptAt), millis(e.CreatedAt))
	if err != nil {
		return domain.OutboxEntry{}, false, err
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return domain.OutboxEntry{}, false, err
	}
	e.Seq = seq
	s.entries[e.Ref] = e
	return e, true, nil
}

// Update writes an entry's mutable state back. SentMsgID is deliberately not
// persisted: it matters only between a successful request and the message
// landing in state, and a crash in that window must re-send rather than assume.
func (s *Store) Update(e domain.OutboxEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(
		`UPDATE outbox SET state = ?, attempts = ?, next_attempt_at = ?, err_kind = ?, err_detail = ?
		 WHERE ref = ?`,
		string(e.State), e.Attempts, millis(e.NextAttemptAt), string(e.ErrKind), e.ErrDetail, e.Ref,
	); err != nil {
		return err
	}
	s.entries[e.Ref] = e
	return nil
}

// Delete drops an entry: it was sent, or the user discarded it.
func (s *Store) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM outbox WHERE ref = ?`, ref); err != nil {
		return err
	}
	delete(s.entries, ref)
	return nil
}

// Get returns one entry by its ref.
func (s *Store) Get(ref string) (domain.OutboxEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[ref]
	return e, ok
}

// All returns every entry in submission order.
func (s *Store) All() []domain.OutboxEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.OutboxEntry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	sortBySeq(out)
	return out
}

// ForChat returns one chat's entries in submission order.
func (s *Store) ForChat(chatID int64) []domain.OutboxEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.OutboxEntry
	for _, e := range s.entries {
		if e.ChatID == chatID {
			out = append(out, e)
		}
	}
	sortBySeq(out)
	return out
}

func sortBySeq(entries []domain.OutboxEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
