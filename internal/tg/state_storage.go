package tg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gotd/td/telegram/updates"
	"github.com/sorokin-vladimir/tele/internal/domain"
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

// GetChannelAccessHash and SetChannelAccessHash implement updates.ChannelAccessHasher.
// updates.Manager needs a channel's access hash to register a channelState for it
// at startup (via ForEachChannels) and to call getChannelDifference. Without
// persistence gotd falls back to an in-memory hasher that is empty on every
// launch, so previously-seen channels are skipped at startup and stay untracked
// until a live message arrives — which causes UpdateChannelTooLong after a long
// idle to be dropped and missed messages to never surface (#119).
func (s *sqliteStateStorage) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (int64, bool, error) {
	var hash int64
	err := s.db.QueryRowContext(ctx,
		`SELECT access_hash FROM channel_access_hash WHERE user_id = ? AND channel_id = ?`, userID, channelID,
	).Scan(&hash)
	if err == nil {
		return hash, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	// Fallback to the chat store. channel_access_hash only fills as gotd re-learns
	// hashes from incoming entities, so a channel with a persisted pts but no
	// learned hash yet (e.g. a quiet news channel) would otherwise be skipped at
	// startup and stall after idle (#119). GetDialogs already stored its access
	// hash on the chat row, so reuse it. GetChannelAccessHash is only called for
	// channel IDs; the peer_type guard keeps a user/group with a colliding raw ID
	// from matching.
	err = s.db.QueryRowContext(ctx,
		`SELECT peer_access_hash FROM chats WHERE id = ? AND peer_type IN (?, ?) AND peer_access_hash != 0`,
		channelID, int(domain.PeerChannel), int(domain.PeerSuperGroup),
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return hash, true, nil
}

func (s *sqliteStateStorage) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO channel_access_hash (user_id, channel_id, access_hash) VALUES (?, ?, ?)`,
		userID, channelID, accessHash,
	)
	return err
}

func (s *sqliteStateStorage) ForEachChannels(ctx context.Context, userID int64, f func(context.Context, int64, int) error) error {
	// Read every row into memory and close the cursor BEFORE invoking f. gotd's
	// manager startup calls GetChannelAccessHash (another query on this same
	// *sql.DB) from inside f; with the pool pinned to a single connection
	// (SetMaxOpenConns(1)) a held cursor would deadlock the nested query.
	type channelPts struct {
		id  int64
		pts int
	}
	var channels []channelPts
	if err := func() error {
		rows, err := s.db.QueryContext(ctx,
			`SELECT channel_id, pts FROM channel_pts WHERE user_id = ?`, userID,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var c channelPts
			if err := rows.Scan(&c.id, &c.pts); err != nil {
				return err
			}
			channels = append(channels, c)
		}
		return rows.Err()
	}(); err != nil {
		return err
	}
	for _, c := range channels {
		if err := f(ctx, c.id, c.pts); err != nil {
			return err
		}
	}
	return nil
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
