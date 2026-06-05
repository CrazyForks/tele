package tg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gotd/td/telegram/updates"
)

type sqliteStateStorage struct {
	db *sql.DB
}

// NewSQLiteStateStorage returns an updates.StateStorage backed by the provided
// SQLite database. Use the same *sql.DB instance as the chat store so both
// share a single connection and file.
func NewSQLiteStateStorage(db *sql.DB) updates.StateStorage {
	return &sqliteStateStorage{db: db}
}

func (s *sqliteStateStorage) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	var st updates.State
	err := s.db.QueryRowContext(ctx,
		`SELECT pts, qts, date, seq FROM update_state WHERE user_id = ?`, userID,
	).Scan(&st.Pts, &st.Qts, &st.Date, &st.Seq)
	if errors.Is(err, sql.ErrNoRows) {
		return updates.State{}, false, nil
	}
	if err != nil {
		return updates.State{}, false, err
	}
	return st, true, nil
}

func (s *sqliteStateStorage) SetState(ctx context.Context, userID int64, state updates.State) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO update_state (user_id, pts, qts, date, seq) VALUES (?, ?, ?, ?, ?)`,
		userID, state.Pts, state.Qts, state.Date, state.Seq,
	)
	return err
}

func (s *sqliteStateStorage) SetPts(ctx context.Context, userID int64, pts int) error {
	return s.updateField(ctx, `UPDATE update_state SET pts = ? WHERE user_id = ?`, pts, userID)
}

func (s *sqliteStateStorage) SetQts(ctx context.Context, userID int64, qts int) error {
	return s.updateField(ctx, `UPDATE update_state SET qts = ? WHERE user_id = ?`, qts, userID)
}

func (s *sqliteStateStorage) SetDate(ctx context.Context, userID int64, date int) error {
	return s.updateField(ctx, `UPDATE update_state SET date = ? WHERE user_id = ?`, date, userID)
}

func (s *sqliteStateStorage) SetSeq(ctx context.Context, userID int64, seq int) error {
	return s.updateField(ctx, `UPDATE update_state SET seq = ? WHERE user_id = ?`, seq, userID)
}

func (s *sqliteStateStorage) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE update_state SET date = ?, seq = ? WHERE user_id = ?`, date, seq, userID,
	)
	if err != nil {
		return err
	}
	return requireRowAffected(res)
}

func (s *sqliteStateStorage) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	var pts int
	err := s.db.QueryRowContext(ctx,
		`SELECT pts FROM channel_pts WHERE user_id = ? AND channel_id = ?`, userID, channelID,
	).Scan(&pts)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return pts, err == nil, err
}

func (s *sqliteStateStorage) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO channel_pts (user_id, channel_id, pts) VALUES (?, ?, ?)`,
		userID, channelID, pts,
	)
	return err
}

func (s *sqliteStateStorage) ForEachChannels(ctx context.Context, userID int64, f func(context.Context, int64, int) error) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel_id, pts FROM channel_pts WHERE user_id = ?`, userID,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var channelID int64
		var pts int
		if err := rows.Scan(&channelID, &pts); err != nil {
			return err
		}
		if err := f(ctx, channelID, pts); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *sqliteStateStorage) updateField(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return requireRowAffected(res)
}

func requireRowAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("state not found")
	}
	return nil
}
