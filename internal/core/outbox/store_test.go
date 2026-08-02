package outbox

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func textEntry(ref string, chatID int64, body string) domain.OutboxEntry {
	return domain.OutboxEntry{
		Ref:       ref,
		ChatID:    chatID,
		RandomID:  RandomIDFor(ref),
		Kind:      domain.OutboxText,
		State:     domain.OutboxQueued,
		Message:   &domain.OutboxMessage{Text: body},
		CreatedAt: time.Unix(1700000000, 0),
	}
}

func TestAdd_AssignsAMonotonicSeq(t *testing.T) {
	s, err := NewStore(openDB(t, filepath.Join(t.TempDir(), "q.db")))
	require.NoError(t, err)

	first, added, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)
	require.True(t, added)
	second, _, err := s.Add(textEntry("b", 10, "two"))
	require.NoError(t, err)

	assert.Greater(t, second.Seq, first.Seq)
}

func TestAdd_IsIdempotentPerRef(t *testing.T) {
	s, err := NewStore(openDB(t, filepath.Join(t.TempDir(), "q.db")))
	require.NoError(t, err)

	_, first, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)
	_, second, err := s.Add(textEntry("a", 10, "one again"))
	require.NoError(t, err)

	assert.True(t, first)
	assert.False(t, second, "a known ref must not create a second entry")
	assert.Len(t, s.All(), 1)
}

func TestSeqIsNeverReused(t *testing.T) {
	// AUTOINCREMENT, not a plain rowid: entries are deleted on success, and a
	// reused id would silently reorder submissions.
	s, err := NewStore(openDB(t, filepath.Join(t.TempDir(), "q.db")))
	require.NoError(t, err)

	first, _, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)
	require.NoError(t, s.Delete("a"))
	second, _, err := s.Add(textEntry("b", 10, "two"))
	require.NoError(t, err)

	assert.Greater(t, second.Seq, first.Seq)
}

func TestReopen_KeepsEntriesAndResetsSending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.db")

	s, err := NewStore(openDB(t, path))
	require.NoError(t, err)
	e, _, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)
	e.State = domain.OutboxSending
	require.NoError(t, s.Update(e))

	reopened, err := NewStore(openDB(t, path))
	require.NoError(t, err)

	got, ok := reopened.Get("a")
	require.True(t, ok)
	assert.Equal(t, domain.OutboxQueued, got.State, "a send in flight when the process died is queued again")
	assert.Equal(t, e.RandomID, got.RandomID, "the random_id must survive: it is what makes the retry safe")
	require.NotNil(t, got.Message)
	assert.Equal(t, "one", got.Message.Text)
	assert.True(t, got.NextAttemptAt.IsZero())
}

func TestForChat_ReturnsOnlyItsOwnInSubmissionOrder(t *testing.T) {
	s, err := NewStore(openDB(t, filepath.Join(t.TempDir(), "q.db")))
	require.NoError(t, err)
	_, _, err = s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)
	_, _, err = s.Add(textEntry("b", 20, "other chat"))
	require.NoError(t, err)
	_, _, err = s.Add(textEntry("c", 10, "two"))
	require.NoError(t, err)

	got := s.ForChat(10)

	require.Len(t, got, 2)
	assert.Equal(t, "one", got[0].Message.Text)
	assert.Equal(t, "two", got[1].Message.Text)
}

func TestUpdate_PersistsFailureDetail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.db")
	s, err := NewStore(openDB(t, path))
	require.NoError(t, err)
	e, _, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)

	e.State = domain.OutboxFailed
	e.ErrKind = "forbidden"
	e.ErrDetail = "CHAT_WRITE_FORBIDDEN"
	e.Attempts = 2
	require.NoError(t, s.Update(e))

	reopened, err := NewStore(openDB(t, path))
	require.NoError(t, err)
	got, ok := reopened.Get("a")
	require.True(t, ok)
	assert.Equal(t, domain.OutboxFailed, got.State)
	assert.Equal(t, "CHAT_WRITE_FORBIDDEN", got.ErrDetail)
	assert.Equal(t, 2, got.Attempts)
}

func TestUpdate_PersistsTheRetryTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q.db")
	s, err := NewStore(openDB(t, path))
	require.NoError(t, err)
	e, _, err := s.Add(textEntry("a", 10, "one"))
	require.NoError(t, err)

	due := time.Unix(1700000123, 0)
	e.NextAttemptAt = due
	require.NoError(t, s.Update(e))

	reopened, err := NewStore(openDB(t, path))
	require.NoError(t, err)
	got, ok := reopened.Get("a")
	require.True(t, ok)
	assert.True(t, got.NextAttemptAt.Equal(due))
}

func TestRandomIDFor_IsDeterministicAndNonZero(t *testing.T) {
	assert.Equal(t, RandomIDFor("abc"), RandomIDFor("abc"))
	assert.NotEqual(t, RandomIDFor("abc"), RandomIDFor("abd"))
	assert.NotZero(t, RandomIDFor("abc"))
}
