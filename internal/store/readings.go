package store

import (
	"context"
	"database/sql"
	"time"

	"dropcatch/internal/alert"
)

type Reading struct {
	VariantID      int64
	At             time.Time
	PriceCents     int64
	BestCents      int64
	OriginalCents  *int64 // the seller's "was" price, display only
	BestIsEstimate bool
	InStock        bool
}

func (s *Store) AddReading(ctx context.Context, r Reading) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO readings (variant_id, at, price_cents, best_cents, original_cents, best_is_estimate, in_stock)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.VariantID, r.At.Unix(), r.PriceCents, r.BestCents, r.OriginalCents, r.BestIsEstimate, r.InStock)
	return err
}

func (s *Store) LogCheck(ctx context.Context, linkID int64, at time.Time, ok bool, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO check_log (link_id, at, ok, error) VALUES (?, ?, ?, ?)`, linkID, at.Unix(), ok, errMsg)
	return err
}

// countable limits readings to variants that count toward prices right now.
const countable = `
	FROM readings r
	JOIN variants v ON v.id = r.variant_id
	JOIN links l ON l.id = v.link_id
	WHERE l.product_id = ? AND v.tracked = 1 AND v.gone = 0`

// DailyCheapest returns, for each Singapore day in [from, to), the lowest best
// price among tracked, present, in stock variants.
func (s *Store) DailyCheapest(ctx context.Context, productID int64, from, to time.Time) ([]alert.DayPrice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT date(r.at + 28800, 'unixepoch') AS day, MIN(r.best_cents)`+countable+`
		 AND r.in_stock = 1 AND r.at >= ? AND r.at < ?
		 GROUP BY day ORDER BY day`,
		productID, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []alert.DayPrice
	for rows.Next() {
		var d alert.DayPrice
		if err := rows.Scan(&d.Day, &d.Cents); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

const readingCols = `r.variant_id, r.at, r.price_cents, r.best_cents, r.original_cents, r.best_is_estimate, r.in_stock`

func scanReading(rows *sql.Rows) (Reading, error) {
	var (
		r    Reading
		at   int64
		orig sql.NullInt64
	)
	err := rows.Scan(&r.VariantID, &at, &r.PriceCents, &r.BestCents, &orig, &r.BestIsEstimate, &r.InStock)
	r.At = time.Unix(at, 0)
	if orig.Valid {
		r.OriginalCents = &orig.Int64
	}
	return r, err
}

// LatestReadings returns the newest reading of every variant of the product,
// tracked or not, keyed by variant id.
func (s *Store) LatestReadings(ctx context.Context, productID int64) (map[int64]Reading, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+readingCols+`
		 FROM readings r
		 JOIN variants v ON v.id = r.variant_id
		 JOIN links l ON l.id = v.link_id
		 WHERE l.product_id = ? AND r.id = (SELECT MAX(id) FROM readings WHERE variant_id = r.variant_id)`,
		productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Reading{}
	for rows.Next() {
		r, err := scanReading(rows)
		if err != nil {
			return nil, err
		}
		out[r.VariantID] = r
	}
	return out, rows.Err()
}

// History returns readings of tracked, present variants since the given time,
// oldest first, keyed by variant id.
func (s *Store) History(ctx context.Context, productID int64, since time.Time) (map[int64][]Reading, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+readingCols+countable+` AND r.at >= ? ORDER BY r.at`, productID, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Reading{}
	for rows.Next() {
		r, err := scanReading(rows)
		if err != nil {
			return nil, err
		}
		out[r.VariantID] = append(out[r.VariantID], r)
	}
	return out, rows.Err()
}

// FirstReadingAt returns the time of the product's first reading of any variant.
func (s *Store) FirstReadingAt(ctx context.Context, productID int64) (time.Time, bool, error) {
	var at sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT MIN(r.at) FROM readings r
		 JOIN variants v ON v.id = r.variant_id
		 JOIN links l ON l.id = v.link_id
		 WHERE l.product_id = ?`, productID).Scan(&at)
	if err != nil || !at.Valid {
		return time.Time{}, false, err
	}
	return time.Unix(at.Int64, 0), true, nil
}

// HasTrackedVariants reports whether anything at all is being tracked.
func (s *Store) HasTrackedVariants(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM variants v
		 JOIN links l ON l.id = v.link_id
		 JOIN products p ON p.id = l.product_id
		 WHERE v.tracked = 1 AND v.gone = 0 AND p.paused = 0`).Scan(&n)
	return n > 0, err
}
