// Package checker reads every link of a product, records prices, and sends
// drop and failure emails.
package checker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"dropcatch/internal/alert"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

type Checker struct {
	Store   *store.Store
	Browser reader.Browser
	Mailer  notify.Mailer
	Notices *notify.Notices
	Now     func() time.Time
	// Sleep pauses between links so traffic looks like a person browsing.
	Sleep func(time.Duration)

	mu sync.Mutex // one product check at a time
}

// Preview reads a pasted link once, without saving anything, so you can pick
// variants. The returned URL is the canonical one to store.
func (c *Checker) Preview(ctx context.Context, rawURL string) (reader.Listing, error) {
	r, canon, err := reader.For(rawURL)
	if err != nil {
		return reader.Listing{}, err
	}
	l, err := r.Read(ctx, c.Browser, canon, nil)
	if err == nil {
		err = reader.Validate(l)
	}
	if err != nil {
		c.noteSession(ctx, string(r.Site()), err)
		return reader.Listing{}, err
	}
	if l.URL == "" {
		l.URL = canon
	}
	// A short link only becomes canonical once the browser followed it.
	if _, again, err := reader.For(l.URL); err == nil {
		l.URL = again
	}
	l.Site = r.Site()
	return l, nil
}

// RunAll checks every product that isn't paused.
func (c *Checker) RunAll(ctx context.Context) error {
	products, err := c.Store.ListProducts(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range products {
		if p.Paused {
			continue
		}
		if err := c.CheckProduct(ctx, p.ID); err != nil {
			errs = append(errs, fmt.Errorf("product %d: %w", p.ID, err))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(errs...)
}

type found struct {
	link    store.Link
	variant store.Variant
	reading store.Reading
}

// CheckProduct reads each link, records prices, then applies the alert rule.
// Read failures are recorded and emailed, not returned. The returned error is
// for the store or email failing.
func (c *Checker) CheckProduct(ctx context.Context, productID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, err := c.Store.GetProduct(ctx, productID)
	if err != nil {
		return err
	}
	links, err := c.Store.ListLinks(ctx, productID)
	if err != nil {
		return err
	}

	var (
		readings []found
		errs     []error
		read     = 0
	)
	for _, l := range links {
		st, err := c.Store.GetSession(ctx, l.Site)
		if err != nil {
			return err
		}
		if st == store.SessionExpired || st == store.SessionBlocked {
			continue // paused until you log in again
		}
		if read > 0 && c.Sleep != nil {
			c.Sleep(0)
		}
		read++
		got, err := c.checkLink(ctx, p, l)
		if err != nil {
			errs = append(errs, err)
		}
		readings = append(readings, got...)
	}

	if err := c.evaluate(ctx, p, readings); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c *Checker) checkLink(ctx context.Context, p store.Product, l store.Link) ([]found, error) {
	now := c.Now()
	variants, err := c.Store.ListVariants(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	var want []string
	for _, v := range variants {
		if v.Tracked && !v.Gone {
			want = append(want, v.ExternalID)
		}
	}

	r, _, err := reader.For(l.URL)
	var listing reader.Listing
	if err == nil {
		listing, err = r.Read(ctx, c.Browser, l.URL, want)
	}
	if err == nil {
		err = reader.Validate(listing)
	}
	if err != nil {
		return nil, c.recordFailure(ctx, p, l, err, now)
	}

	if err := c.Store.SetLinkResult(ctx, l.ID, store.LinkOK, "", now); err != nil {
		return nil, err
	}
	_ = c.Store.LogCheck(ctx, l.ID, now, true, "")
	if listing.Title != "" && (listing.Title != l.Title || listing.Seller != l.Seller) {
		_ = c.Store.UpdateLinkInfo(ctx, l.ID, listing.Title, listing.Seller)
	}
	_ = c.Store.SetSession(ctx, l.Site, store.SessionOK)
	_ = c.Store.SetTime(ctx, "last_success_at", now)
	c.clear(ctx, "link:"+strconv.FormatInt(l.ID, 10))
	c.clear(ctx, "session:"+l.Site)

	return c.reconcile(ctx, l, variants, listing, now)
}

// reconcile matches the page's variants to stored ones and records readings
// for the tracked ones.
func (c *Checker) reconcile(ctx context.Context, l store.Link, stored []store.Variant, listing reader.Listing, now time.Time) ([]found, error) {
	byID := map[string]store.Variant{}
	for _, v := range stored {
		byID[v.ExternalID] = v
	}
	seen := map[string]bool{}
	var out []found
	for _, vp := range listing.Variants {
		seen[vp.ExternalID] = true
		v, known := byID[vp.ExternalID]
		if !known {
			v = store.Variant{LinkID: l.ID, ExternalID: vp.ExternalID, IsNew: true}
		}
		v.Name, v.Gone = vp.Name, false
		v, err := c.Store.UpsertVariant(ctx, v)
		if err != nil {
			return out, err
		}
		if !v.Tracked {
			continue
		}
		rd := store.Reading{
			VariantID: v.ID, At: now,
			PriceCents: vp.PriceCents, BestCents: vp.BestCents,
			BestIsEstimate: vp.BestIsEstimate, InStock: vp.InStock,
		}
		if vp.OriginalCents > 0 {
			orig := vp.OriginalCents
			rd.OriginalCents = &orig
		}
		if err := c.Store.AddReading(ctx, rd); err != nil {
			return out, err
		}
		out = append(out, found{link: l, variant: v, reading: rd})
	}
	for _, v := range stored {
		if !seen[v.ExternalID] && !v.Gone {
			v.Gone = true
			if _, err := c.Store.UpsertVariant(ctx, v); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func (c *Checker) recordFailure(ctx context.Context, p store.Product, l store.Link, readErr error, now time.Time) error {
	_ = c.Store.LogCheck(ctx, l.ID, now, false, readErr.Error())
	if c.noteSession(ctx, l.Site, readErr) {
		return nil
	}
	log.Printf("check %s: %v", l.URL, readErr)
	if err := c.Store.SetLinkResult(ctx, l.ID, store.LinkBroken, readErr.Error(), now); err != nil {
		return err
	}
	subject, text := notify.BrokenLinkEmail(p.Name, l.URL, readErr.Error())
	return c.raise(ctx, "link:"+strconv.FormatInt(l.ID, 10), subject, text)
}

// noteSession records a login or verification problem for the site and
// emails once. It reports whether err was such a problem.
func (c *Checker) noteSession(ctx context.Context, site string, err error) bool {
	var st store.SessionStatus
	switch {
	case errors.Is(err, reader.ErrLoggedOut):
		st = store.SessionExpired
	case errors.Is(err, reader.ErrBlocked):
		st = store.SessionBlocked
	default:
		return false
	}
	log.Printf("%s session: %s", site, st)
	_ = c.Store.SetSession(ctx, site, st)
	subject, text := notify.SessionEmail(site, st == store.SessionBlocked)
	if err := c.raise(ctx, "session:"+site, subject, text); err != nil {
		log.Printf("session email: %v", err)
	}
	return true
}

func (c *Checker) raise(ctx context.Context, key, subject, text string) error {
	if c.Notices == nil {
		return nil
	}
	return c.Notices.Raise(ctx, key, subject, text)
}

func (c *Checker) clear(ctx context.Context, key string) {
	if c.Notices != nil {
		_ = c.Notices.Clear(ctx, key)
	}
}

// Params returns the product's alert settings, falling back to the globals.
func Params(p store.Product, s store.Settings) alert.Params {
	out := alert.Params{DropPct: s.DropPct, WindowDays: s.WindowDays, MinHistoryDays: s.MinHistoryDays}
	if p.DropPct != nil {
		out.DropPct = *p.DropPct
	}
	if p.WindowDays != nil {
		out.WindowDays = *p.WindowDays
	}
	if p.MinHistoryDays != nil {
		out.MinHistoryDays = *p.MinHistoryDays
	}
	return out
}

// AlertInput builds the alert rule's input from stored history. The server
// uses it too, so the dashboard's typical price matches the emails.
func AlertInput(ctx context.Context, s *store.Store, p store.Product, now time.Time, current *int64) (alert.Input, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return alert.Input{}, err
	}
	params := Params(p, settings)
	today := alert.DayStart(now)
	daily, err := s.DailyCheapest(ctx, p.ID, today.AddDate(0, 0, -params.WindowDays), today)
	if err != nil {
		return alert.Input{}, err
	}
	in := alert.Input{Now: now, CurrentCents: current, Daily: daily, Params: params, LastAlertedCents: p.LastAlertedCents}
	if first, ok, err := s.FirstReadingAt(ctx, p.ID); err != nil {
		return in, err
	} else if ok {
		in.FirstDay = first.In(alert.SGT).Format("2006-01-02")
	}
	return in, nil
}

func (c *Checker) evaluate(ctx context.Context, p store.Product, readings []found) error {
	var best *found
	for i := range readings {
		r := &readings[i]
		if r.reading.InStock && (best == nil || r.reading.BestCents < best.reading.BestCents) {
			best = r
		}
	}
	var current *int64
	if best != nil {
		current = &best.reading.BestCents
	}
	now := c.Now()
	in, err := AlertInput(ctx, c.Store, p, now, current)
	if err != nil {
		return err
	}
	res := alert.Evaluate(in)

	if res.Alert {
		subject, text := notify.DropEmail(notify.DropAlert{
			Product: p.Name, Site: best.link.Site, Seller: best.link.Seller, Variant: best.variant.Name,
			URL: best.link.URL, NewCents: best.reading.BestCents, TypicalCents: res.TypicalCents,
			DropPct: res.DropPct, HistoryDays: res.HistoryDays, WindowDays: in.Params.WindowDays,
		})
		settings, err := c.Store.GetSettings(ctx)
		if err != nil {
			return err
		}
		// Leave the no spam state alone when the email fails, so it retries.
		if err := c.Mailer.Send(ctx, settings.Email, subject, text); err != nil {
			return fmt.Errorf("drop email: %w", err)
		}
		log.Printf("drop email sent: %s", subject)
	}
	if !samePtr(p.LastAlertedCents, res.LastAlertedCents) {
		return c.Store.SetLastAlerted(ctx, p.ID, res.LastAlertedCents)
	}
	return nil
}

func samePtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
