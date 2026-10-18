package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Product struct {
	ID     int64
	Name   string
	Paused bool
	// Per-product overrides. Nil means use the global setting.
	DropPct        *float64
	WindowDays     *int
	MinHistoryDays *int
	// The price of the last drop email, for the no spam rule.
	LastAlertedCents *int64
	CreatedAt        time.Time
}

type LinkStatus string

const (
	LinkOK      LinkStatus = "ok"
	LinkBroken  LinkStatus = "broken"
	LinkPending LinkStatus = "pending"
)

type Link struct {
	ID            int64
	ProductID     int64
	URL           string
	Site          string
	Title         string
	Seller        string
	Status        LinkStatus
	LastError     string
	LastCheckedAt *time.Time
}

type Variant struct {
	ID         int64
	LinkID     int64
	ExternalID string
	Name       string
	Tracked    bool
	IsNew      bool
	Gone       bool
}

const productCols = `id, name, paused, drop_pct, window_days, min_history_days, last_alerted_cents, created_at`

func scanProduct(row interface{ Scan(...any) error }) (Product, error) {
	var (
		p                      Product
		drop                   sql.NullFloat64
		window, minHist, alert sql.NullInt64
		created                int64
	)
	err := row.Scan(&p.ID, &p.Name, &p.Paused, &drop, &window, &minHist, &alert, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if drop.Valid {
		p.DropPct = &drop.Float64
	}
	if window.Valid {
		n := int(window.Int64)
		p.WindowDays = &n
	}
	if minHist.Valid {
		n := int(minHist.Int64)
		p.MinHistoryDays = &n
	}
	if alert.Valid {
		p.LastAlertedCents = &alert.Int64
	}
	p.CreatedAt = time.Unix(created, 0)
	return p, err
}

func (s *Store) CreateProduct(ctx context.Context, name string) (Product, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO products (name, created_at) VALUES (?, ?)`, name, time.Now().Unix())
	if err != nil {
		return Product{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetProduct(ctx, id)
}

func (s *Store) GetProduct(ctx context.Context, id int64) (Product, error) {
	return scanProduct(s.db.QueryRowContext(ctx, `SELECT `+productCols+` FROM products WHERE id = ?`, id))
}

func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+productCols+` FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateProduct(ctx context.Context, p Product) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE products SET name = ?, paused = ?, drop_pct = ?, window_days = ?, min_history_days = ?, last_alerted_cents = ?
		 WHERE id = ?`,
		p.Name, p.Paused, p.DropPct, p.WindowDays, p.MinHistoryDays, p.LastAlertedCents, p.ID)
	return mustAffect(res, err)
}

func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	return mustAffect(s.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, id))
}

func mustAffect(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const linkCols = `id, product_id, url, site, title, seller, status, last_error, last_checked_at`

func scanLink(row interface{ Scan(...any) error }) (Link, error) {
	var (
		l       Link
		status  string
		checked sql.NullInt64
	)
	err := row.Scan(&l.ID, &l.ProductID, &l.URL, &l.Site, &l.Title, &l.Seller, &status, &l.LastError, &checked)
	if errors.Is(err, sql.ErrNoRows) {
		return l, ErrNotFound
	}
	l.Status = LinkStatus(status)
	l.LastCheckedAt = unixPtr(checked)
	return l, err
}

func (s *Store) CreateLink(ctx context.Context, productID int64, url, site, title, seller string) (Link, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO links (product_id, url, site, title, seller) VALUES (?, ?, ?, ?, ?)`,
		productID, url, site, title, seller)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Link{}, ErrDuplicate
		}
		return Link{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetLink(ctx, id)
}

func (s *Store) GetLink(ctx context.Context, id int64) (Link, error) {
	return scanLink(s.db.QueryRowContext(ctx, `SELECT `+linkCols+` FROM links WHERE id = ?`, id))
}

func (s *Store) ListLinks(ctx context.Context, productID int64) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+linkCols+` FROM links WHERE product_id = ? ORDER BY id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// LatestLinkURL returns the URL of the site's most recently checked link, or
// "" when you track nothing there.
func (s *Store) LatestLinkURL(ctx context.Context, site string) (string, error) {
	var u string
	err := s.db.QueryRowContext(ctx,
		`SELECT url FROM links WHERE site = ? ORDER BY COALESCE(last_checked_at, 0) DESC, id DESC LIMIT 1`, site).Scan(&u)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return u, err
}

// UpdateLinkInfo refreshes the title and seller read from the page.
func (s *Store) UpdateLinkInfo(ctx context.Context, linkID int64, title, seller string) error {
	return mustAffect(s.db.ExecContext(ctx, `UPDATE links SET title = ?, seller = ? WHERE id = ?`, title, seller, linkID))
}

func (s *Store) SetLinkResult(ctx context.Context, linkID int64, status LinkStatus, errMsg string, at time.Time) error {
	return mustAffect(s.db.ExecContext(ctx,
		`UPDATE links SET status = ?, last_error = ?, last_checked_at = ? WHERE id = ?`,
		string(status), errMsg, at.Unix(), linkID))
}

func (s *Store) DeleteLink(ctx context.Context, id int64) error {
	return mustAffect(s.db.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, id))
}

const variantCols = `id, link_id, external_id, name, tracked, is_new, gone`

func scanVariant(row interface{ Scan(...any) error }) (Variant, error) {
	var v Variant
	err := row.Scan(&v.ID, &v.LinkID, &v.ExternalID, &v.Name, &v.Tracked, &v.IsNew, &v.Gone)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// UpsertVariant inserts a variant, or for an existing (link, external id)
// updates its name and gone flag. Tracked and IsNew only apply on insert, so a
// re-read never changes what you chose to track.
func (s *Store) UpsertVariant(ctx context.Context, v Variant) (Variant, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO variants (link_id, external_id, name, tracked, is_new, gone) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (link_id, external_id) DO UPDATE SET name = excluded.name, gone = excluded.gone`,
		v.LinkID, v.ExternalID, v.Name, v.Tracked, v.IsNew, v.Gone)
	if err != nil {
		return Variant{}, err
	}
	return scanVariant(s.db.QueryRowContext(ctx,
		`SELECT `+variantCols+` FROM variants WHERE link_id = ? AND external_id = ?`, v.LinkID, v.ExternalID))
}

func (s *Store) GetVariant(ctx context.Context, id int64) (Variant, error) {
	return scanVariant(s.db.QueryRowContext(ctx, `SELECT `+variantCols+` FROM variants WHERE id = ?`, id))
}

func (s *Store) ListVariants(ctx context.Context, linkID int64) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+variantCols+` FROM variants WHERE link_id = ? ORDER BY id`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Variant
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetVariantTracked also clears the "new variant" badge, since you've now
// looked at it.
func (s *Store) SetVariantTracked(ctx context.Context, id int64, tracked bool) error {
	return mustAffect(s.db.ExecContext(ctx, `UPDATE variants SET tracked = ?, is_new = 0 WHERE id = ?`, tracked, id))
}

// SetLastAlerted stores the no spam state without touching your settings.
func (s *Store) SetLastAlerted(ctx context.Context, productID int64, cents *int64) error {
	return mustAffect(s.db.ExecContext(ctx, `UPDATE products SET last_alerted_cents = ? WHERE id = ?`, cents, productID))
}
