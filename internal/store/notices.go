package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// NoticeActive reports whether a one-shot failure email for key was sent and
// the problem hasn't cleared since.
func (s *Store) NoticeActive(ctx context.Context, key string) (bool, error) {
	var active bool
	err := s.db.QueryRowContext(ctx, `SELECT active FROM notices WHERE key = ?`, key).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return active, err
}

func (s *Store) SetNotice(ctx context.Context, key string, active bool, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notices (key, active, sent_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET active = excluded.active, sent_at = excluded.sent_at`,
		key, active, at.Unix())
	return err
}
