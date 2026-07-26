package notices

import (
	"database/sql"
	"strconv"
	"time"
)

// Seen records which notices have already been shown.
//
// This deliberately does not live on store.Store: that interface is already
// wide, and seen-state is app-level preference data rather than account state.
type Seen interface {
	IsSeen(id string) bool
	MarkSeen(id string)
}

// keyPrefix namespaces notice rows inside the shared metadata table.
const keyPrefix = "notice:"

type sqliteSeen struct{ db *sql.DB }

// NewSQLiteSeen stores seen-state in the existing metadata key/value table.
func NewSQLiteSeen(db *sql.DB) Seen { return &sqliteSeen{db: db} }

func (s *sqliteSeen) IsSeen(id string) bool {
	var v string
	err := s.db.QueryRow(`SELECT value FROM metadata WHERE key = ?`, keyPrefix+id).Scan(&v)
	return err == nil
}

// MarkSeen is best effort: failing to record a notice means showing it once
// more, which is strictly better than failing startup over it.
func (s *sqliteSeen) MarkSeen(id string) {
	_, _ = s.db.Exec(
		`INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)`,
		keyPrefix+id, strconv.FormatInt(time.Now().Unix(), 10),
	)
}

type memorySeen struct{ ids map[string]bool }

// NewMemorySeen is the non-persistent implementation used by tests.
func NewMemorySeen() Seen { return &memorySeen{ids: map[string]bool{}} }

func (m *memorySeen) IsSeen(id string) bool { return m.ids[id] }
func (m *memorySeen) MarkSeen(id string)    { m.ids[id] = true }
