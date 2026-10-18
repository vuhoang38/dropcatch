package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seed creates a product with one link and the given variants.
func seed(t *testing.T, s *Store, variants ...Variant) (Product, Link, []Variant) {
	t.Helper()
	ctx := context.Background()
	p, err := s.CreateProduct(ctx, "AirPods Pro 3")
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.CreateLink(ctx, p.ID, "https://shopee.sg/product/1/2", "shopee", "AirPods", "Apple")
	if err != nil {
		t.Fatal(err)
	}
	var out []Variant
	for _, v := range variants {
		v.LinkID = l.ID
		v, err := s.UpsertVariant(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return p, l, out
}

func TestSettingsDefaults(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	got, err := s.GetSettings(ctx)
	if err != nil || got != DefaultSettings() {
		t.Fatalf("GetSettings = %+v, %v", got, err)
	}
	if got.DropPct != 20 || got.WindowDays != 30 || got.MinHistoryDays != 2 || got.CheckIntervalHours != 3 {
		t.Fatalf("defaults = %+v", got)
	}
	got.DropPct = 15
	if err := s.PutSettings(ctx, got); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.GetSettings(ctx); again.DropPct != 15 {
		t.Fatalf("DropPct = %v", again.DropPct)
	}
}

func TestTimes(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if got, err := s.GetTime(ctx, "last_run_at"); err != nil || !got.IsZero() {
		t.Fatalf("unset time = %v, %v", got, err)
	}
	now := time.Unix(1792284369, 0)
	if err := s.SetTime(ctx, "last_run_at", now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetTime(ctx, "last_run_at"); !got.Equal(now) {
		t.Fatalf("time = %v", got)
	}
}

func TestLinkDuplicate(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, l, _ := seed(t, s)
	_, err := s.CreateLink(ctx, p.ID, l.URL, "shopee", "", "")
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err = %v, want ErrDuplicate", err)
	}
}

func TestUpsertVariantKeepsTracked(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	_, l, vs := seed(t, s, Variant{ExternalID: "m1", Name: "Pro 3", Tracked: true})
	got, err := s.UpsertVariant(ctx, Variant{LinkID: l.ID, ExternalID: "m1", Name: "Pro 3 USB-C", Gone: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != vs[0].ID || !got.Tracked || !got.Gone || got.Name != "Pro 3 USB-C" {
		t.Fatalf("upsert = %+v", got)
	}
}

func TestSetVariantTrackedClearsNew(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	_, l, vs := seed(t, s, Variant{ExternalID: "m1", IsNew: true})
	if err := s.SetVariantTracked(ctx, vs[0].ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListVariants(ctx, l.ID)
	if !got[0].Tracked || got[0].IsNew {
		t.Fatalf("variant = %+v", got[0])
	}
}

func reading(v Variant, at string, best int64, inStock bool) Reading {
	ts, _ := time.Parse(time.RFC3339, at)
	return Reading{VariantID: v.ID, At: ts, PriceCents: best, BestCents: best, InStock: inStock}
}

func TestDailyCheapestSingaporeDay(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, vs := seed(t, s, Variant{ExternalID: "m1", Tracked: true})
	for _, r := range []Reading{
		reading(vs[0], "2026-10-17T23:30:00Z", 30000, true), // 07:30 SGT on the 18th
		reading(vs[0], "2026-10-18T03:00:00Z", 28000, true),
	} {
		if err := s.AddReading(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	from, _ := time.Parse(time.RFC3339, "2026-10-01T00:00:00+08:00")
	to, _ := time.Parse(time.RFC3339, "2026-10-19T00:00:00+08:00")
	got, err := s.DailyCheapest(ctx, p.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Day != "2026-10-18" || got[0].Cents != 28000 {
		t.Fatalf("daily = %+v", got)
	}
}

func TestDailyCheapestFilters(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, vs := seed(t, s,
		Variant{ExternalID: "tracked", Tracked: true},
		Variant{ExternalID: "untracked"},
		Variant{ExternalID: "gone", Tracked: true, Gone: true},
	)
	for _, r := range []Reading{
		reading(vs[0], "2026-10-18T03:00:00Z", 10000, false), // out of stock
		reading(vs[1], "2026-10-18T03:00:00Z", 10000, true),  // untracked
		reading(vs[2], "2026-10-18T03:00:00Z", 10000, true),  // gone
		reading(vs[0], "2026-10-18T04:00:00Z", 25000, true),
	} {
		if err := s.AddReading(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	from, _ := time.Parse(time.RFC3339, "2026-10-01T00:00:00+08:00")
	to, _ := time.Parse(time.RFC3339, "2026-10-19T00:00:00+08:00")
	got, _ := s.DailyCheapest(ctx, p.ID, from, to)
	if len(got) != 1 || got[0].Cents != 25000 {
		t.Fatalf("daily = %+v", got)
	}
}

func TestLatestAndHistory(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, vs := seed(t, s, Variant{ExternalID: "m1", Tracked: true})
	orig := int64(34900)
	r1 := reading(vs[0], "2026-10-17T03:00:00Z", 30000, true)
	r2 := reading(vs[0], "2026-10-18T03:00:00Z", 28000, true)
	r2.OriginalCents = &orig
	r2.BestIsEstimate = true
	for _, r := range []Reading{r1, r2} {
		if err := s.AddReading(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := s.LatestReadings(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := latest[vs[0].ID]
	if got.BestCents != 28000 || got.OriginalCents == nil || *got.OriginalCents != 34900 || !got.BestIsEstimate {
		t.Fatalf("latest = %+v", got)
	}
	hist, _ := s.History(ctx, p.ID, r1.At)
	if len(hist[vs[0].ID]) != 2 || hist[vs[0].ID][0].BestCents != 30000 {
		t.Fatalf("history = %+v", hist)
	}
	first, ok, _ := s.FirstReadingAt(ctx, p.ID)
	if !ok || !first.Equal(r1.At) {
		t.Fatalf("first = %v %v", first, ok)
	}
}

func TestSessions(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if st, _ := s.GetSession(ctx, "shopee"); st != SessionUnknown {
		t.Fatalf("unset session = %q", st)
	}
	_ = s.SetSession(ctx, "shopee", SessionExpired)
	if st, _ := s.GetSession(ctx, "shopee"); st != SessionExpired {
		t.Fatalf("session = %q", st)
	}
}

func TestNoticeRoundTrip(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	now := time.Now()
	if err := s.SetNotice(ctx, "stale", true, now); err != nil {
		t.Fatal(err)
	}
	if on, _ := s.NoticeActive(ctx, "stale"); !on {
		t.Fatal("want active")
	}
	_ = s.SetNotice(ctx, "stale", false, now)
	if on, _ := s.NoticeActive(ctx, "stale"); on {
		t.Fatal("want inactive")
	}
}

func TestDeleteProductCascades(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, l, vs := seed(t, s, Variant{ExternalID: "m1", Tracked: true})
	_ = s.AddReading(ctx, reading(vs[0], "2026-10-18T03:00:00Z", 28000, true))
	if err := s.DeleteProduct(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, l.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("link err = %v", err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM readings`).Scan(&n)
	if n != 0 {
		t.Fatalf("readings left = %d", n)
	}
}

func TestProductOverridesRoundTrip(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, _ := seed(t, s)
	drop, last := 15.0, int64(22900)
	p.DropPct, p.LastAlertedCents, p.Paused = &drop, &last, true
	if err := s.UpdateProduct(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProduct(ctx, p.ID)
	if got.DropPct == nil || *got.DropPct != 15 || got.WindowDays != nil || *got.LastAlertedCents != 22900 || !got.Paused {
		t.Fatalf("product = %+v", got)
	}
}

func TestSetLastAlertedOnlyTouchesThatField(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, _ := seed(t, s)
	p.Paused = true
	_ = s.UpdateProduct(ctx, p)
	v := int64(22900)
	if err := s.SetLastAlerted(ctx, p.ID, &v); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProduct(ctx, p.ID)
	if !got.Paused || got.LastAlertedCents == nil || *got.LastAlertedCents != 22900 {
		t.Fatalf("product = %+v", got)
	}
	_ = s.SetLastAlerted(ctx, p.ID, nil)
	if got, _ := s.GetProduct(ctx, p.ID); got.LastAlertedCents != nil {
		t.Fatalf("product = %+v", got)
	}
}

func TestLatestReadingsByTimeNotInsertOrder(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _, vs := seed(t, s, Variant{ExternalID: "m1", Tracked: true})
	_ = s.AddReading(ctx, reading(vs[0], "2026-10-18T03:00:00Z", 28000, true))
	_ = s.AddReading(ctx, reading(vs[0], "2026-10-10T03:00:00Z", 30000, true)) // backfilled later
	latest, _ := s.LatestReadings(ctx, p.ID)
	if got := latest[vs[0].ID].BestCents; got != 28000 {
		t.Fatalf("latest = %d, want 28000", got)
	}
}
