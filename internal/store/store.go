// Package store keeps products, links, variants and price readings in SQLite.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("already exists")
)

type Store struct {
	db *sql.DB
}

// Open opens or creates the database at path. ":memory:" works for tests.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if path != ":memory:" {
		dsn += "&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection keeps ":memory:" a single database and avoids writer
	// contention. This app does very little concurrent work.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

type Settings struct {
	DropPct            float64 `json:"dropPct"`
	WindowDays         int     `json:"windowDays"`
	MinHistoryDays     int     `json:"minHistoryDays"`
	CheckIntervalHours int     `json:"checkIntervalHours"`
	ResendAPIKey       string  `json:"resendApiKey"`
	Email              string  `json:"email"`
}

func DefaultSettings() Settings {
	return Settings{DropPct: 20, WindowDays: 30, MinHistoryDays: 2, CheckIntervalHours: 3}
}

func (s *Store) getKV(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) setKV(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	out := DefaultSettings()
	v, ok, err := s.getKV(ctx, "settings")
	if err != nil || !ok {
		return out, err
	}
	return out, json.Unmarshal([]byte(v), &out)
}

func (s *Store) PutSettings(ctx context.Context, in Settings) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return s.setKV(ctx, "settings", string(b))
}

// GetTime returns the zero time when key is unset.
func (s *Store) GetTime(ctx context.Context, key string) (time.Time, error) {
	v, ok, err := s.getKV(ctx, "time:"+key)
	if err != nil || !ok {
		return time.Time{}, err
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(n, 0), nil
}

func (s *Store) SetTime(ctx context.Context, key string, t time.Time) error {
	return s.setKV(ctx, "time:"+key, strconv.FormatInt(t.Unix(), 10))
}

type SessionStatus string

const (
	SessionOK      SessionStatus = "ok"
	SessionExpired SessionStatus = "expired"
	SessionBlocked SessionStatus = "blocked"
	SessionUnknown SessionStatus = "unknown"
)

func (s *Store) GetSession(ctx context.Context, site string) (SessionStatus, error) {
	var st string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM sessions WHERE site = ?`, site).Scan(&st)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionUnknown, nil
	}
	return SessionStatus(st), err
}

func (s *Store) SetSession(ctx context.Context, site string, st SessionStatus) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (site, status, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (site) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at`,
		site, string(st), time.Now().Unix())
	return err
}

func unixPtr(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.Unix(n.Int64, 0)
	return &t
}
