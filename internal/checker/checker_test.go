package checker

import (
	"context"
	"strings"
	"testing"
	"time"

	"dropcatch/internal/alert"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

// fakeReader handles https://fake.test/... and returns whatever next says.
type fakeReader struct {
	calls int
	next  func(url string) (reader.Listing, error)
}

func (f *fakeReader) Site() reader.Site { return "fake" }
func (f *fakeReader) Canonical(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "https://fake.test/") {
		return "", false
	}
	return strings.SplitN(raw, "?", 2)[0], true
}
func (f *fakeReader) Read(ctx context.Context, b reader.Browser, url string, want []string) (reader.Listing, error) {
	f.calls++
	return f.next(url)
}

type mailbox struct{ subjects []string }

func (m *mailbox) Send(ctx context.Context, to, subject, text string) error {
	m.subjects = append(m.subjects, subject)
	return nil
}

var now = time.Date(2026, 10, 18, 10, 0, 0, 0, alert.SGT)

type env struct {
	c    *Checker
	s    *store.Store
	r    *fakeReader
	mail *mailbox
	p    store.Product
	l    store.Link
	v    store.Variant
}

func listing(variants ...reader.VariantPrice) reader.Listing {
	return reader.Listing{Site: "fake", Title: "AirPods Pro 3", Seller: "Apple", Variants: variants}
}

func vp(id string, best int64, inStock bool) reader.VariantPrice {
	return reader.VariantPrice{ExternalID: id, Name: "v" + id, PriceCents: best, BestCents: best, InStock: inStock}
}

// setup makes a product with one link and one tracked variant "a".
func setup(t *testing.T) *env {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	set := store.DefaultSettings()
	set.Email = "me@x.com"
	_ = s.PutSettings(ctx, set)

	r := &fakeReader{next: func(string) (reader.Listing, error) { return listing(vp("a", 25000, true)), nil }}
	reader.Register(r)
	mail := &mailbox{}
	c := &Checker{
		Store: s, Mailer: mail, Now: func() time.Time { return now }, Sleep: func(time.Duration) {},
		Notices: &notify.Notices{Store: s, Mailer: mail, Now: func() time.Time { return now },
			To: func(context.Context) (string, error) { return "me@x.com", nil }},
	}
	p, _ := s.CreateProduct(ctx, "AirPods Pro 3")
	l, _ := s.CreateLink(ctx, p.ID, "https://fake.test/1", "fake", "AirPods Pro 3", "Apple")
	v, _ := s.UpsertVariant(ctx, store.Variant{LinkID: l.ID, ExternalID: "a", Name: "va", Tracked: true})
	return &env{c: c, s: s, r: r, mail: mail, p: p, l: l, v: v}
}

func (e *env) check(t *testing.T) {
	t.Helper()
	if err := e.c.CheckProduct(context.Background(), e.p.ID); err != nil {
		t.Fatal(err)
	}
}

// failUntilBroken runs enough failing checks to mark the link broken.
func (e *env) failUntilBroken(t *testing.T) {
	t.Helper()
	for range BrokenAfter {
		e.check(t)
	}
}

func (e *env) seedDays(t *testing.T, days int, cents int64) {
	t.Helper()
	for ago := days; ago >= 1; ago-- {
		at := now.AddDate(0, 0, -ago)
		_ = e.s.AddReading(context.Background(), store.Reading{VariantID: e.v.ID, At: at, PriceCents: cents, BestCents: cents, InStock: true})
	}
}

func TestCheckRecordsReadings(t *testing.T) {
	e := setup(t)
	e.check(t)
	latest, _ := e.s.LatestReadings(context.Background(), e.p.ID)
	if r, ok := latest[e.v.ID]; !ok || r.BestCents != 25000 || !r.At.Equal(now) {
		t.Fatalf("latest = %+v", latest)
	}
	l, _ := e.s.GetLink(context.Background(), e.l.ID)
	if l.Status != store.LinkOK {
		t.Fatalf("link = %+v", l)
	}
	if last, _ := e.s.GetTime(context.Background(), "last_success_at"); !last.Equal(now) {
		t.Fatalf("last_success_at = %v", last)
	}
}

func TestNewVariantUntracked(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 25000, true), vp("b", 100, true)), nil }
	e.check(t)
	vs, _ := e.s.ListVariants(context.Background(), e.l.ID)
	if len(vs) != 2 || vs[1].ExternalID != "b" || vs[1].Tracked || !vs[1].IsNew {
		t.Fatalf("variants = %+v", vs)
	}
	latest, _ := e.s.LatestReadings(context.Background(), e.p.ID)
	if _, ok := latest[vs[1].ID]; ok {
		t.Fatal("untracked variant got a reading")
	}
}

func TestGoneVariant(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("z", 25000, true)), nil }
	e.check(t)
	v, _ := e.s.GetVariant(context.Background(), e.v.ID)
	if !v.Gone || !v.Tracked {
		t.Fatalf("variant = %+v", v)
	}
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 25000, true)), nil }
	e.check(t)
	if v, _ := e.s.GetVariant(context.Background(), e.v.ID); v.Gone {
		t.Fatal("variant came back but is still gone")
	}
}

func TestBrokenLinkEmailsOnce(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrUnreadable }
	e.failUntilBroken(t)
	e.check(t)
	l, _ := e.s.GetLink(context.Background(), e.l.ID)
	if l.Status != store.LinkBroken || !strings.Contains(l.LastError, "price not found") {
		t.Fatalf("link = %+v", l)
	}
	if len(e.mail.subjects) != 1 || !strings.Contains(e.mail.subjects[0], "stopped working") {
		t.Fatalf("mail = %v", e.mail.subjects)
	}
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 25000, true)), nil }
	e.check(t)
	if on, _ := e.s.NoticeActive(context.Background(), "link:1"); on {
		t.Fatal("notice should clear after recovery")
	}
}

func TestOneMissIsNotBroken(t *testing.T) {
	e := setup(t)
	fail := func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrUnreadable }
	ok := func(string) (reader.Listing, error) { return listing(vp("a", 25000, true)), nil }

	e.check(t) // ok, so the link starts out working
	e.r.next = fail
	for range BrokenAfter - 1 {
		e.check(t)
	}
	e.r.next = ok
	e.check(t)
	e.r.next = fail
	for range BrokenAfter - 1 {
		e.check(t)
	}
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status != store.LinkOK {
		t.Fatalf("a good check should restart the count: link = %+v", l)
	}
	if len(e.mail.subjects) != 0 {
		t.Fatalf("mail = %v", e.mail.subjects)
	}
	e.check(t)
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status != store.LinkBroken {
		t.Fatalf("link = %+v", l)
	}
}

func TestInvalidListingIsBroken(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 0, true)), nil }
	e.failUntilBroken(t)
	latest, _ := e.s.LatestReadings(context.Background(), e.p.ID)
	if len(latest) != 0 {
		t.Fatalf("zero price was recorded: %+v", latest)
	}
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status != store.LinkBroken {
		t.Fatalf("link = %+v", l)
	}
}

func TestSessionExpiredSkipsSite(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrLoggedOut }
	e.check(t)
	if st, _ := e.s.GetSession(context.Background(), "fake"); st != store.SessionExpired {
		t.Fatalf("session = %q", st)
	}
	e.check(t)
	if e.r.calls != 1 {
		t.Fatalf("reader called %d times, want 1", e.r.calls)
	}
	if len(e.mail.subjects) != 1 || !strings.Contains(e.mail.subjects[0], "login expired") {
		t.Fatalf("mail = %v", e.mail.subjects)
	}
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status == store.LinkBroken {
		t.Fatal("a login problem should not mark the link broken")
	}
}

func TestBlockedSession(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrBlocked }
	e.check(t)
	if st, _ := e.s.GetSession(context.Background(), "fake"); st != store.SessionBlocked {
		t.Fatalf("session = %q", st)
	}
}

func TestDropAlertSends(t *testing.T) {
	e := setup(t)
	drop := 15.0
	e.p.DropPct = &drop
	_ = e.s.UpdateProduct(context.Background(), e.p)
	e.seedDays(t, 5, 27900)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 22900, true)), nil }
	e.check(t)
	if len(e.mail.subjects) != 1 || e.mail.subjects[0] != "AirPods Pro 3 dropped to $229.00 (18% under typical)" {
		t.Fatalf("mail = %v", e.mail.subjects)
	}
	p, _ := e.s.GetProduct(context.Background(), e.p.ID)
	if p.LastAlertedCents == nil || *p.LastAlertedCents != 22900 {
		t.Fatalf("product = %+v", p)
	}
	e.check(t)
	if len(e.mail.subjects) != 1 {
		t.Fatalf("second check at the same price emailed again: %v", e.mail.subjects)
	}
}

func TestAllLinksFailNoReset(t *testing.T) {
	e := setup(t)
	last := int64(20000)
	e.p.LastAlertedCents = &last
	_ = e.s.UpdateProduct(context.Background(), e.p)
	e.seedDays(t, 5, 27900)
	e.r.next = func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrUnreadable }
	e.check(t)
	p, _ := e.s.GetProduct(context.Background(), e.p.ID)
	if p.LastAlertedCents == nil || *p.LastAlertedCents != 20000 {
		t.Fatalf("product = %+v", p)
	}
	for _, s := range e.mail.subjects {
		if strings.Contains(s, "dropped") {
			t.Fatalf("drop email sent: %v", e.mail.subjects)
		}
	}
}

func TestOutOfStockIgnored(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_, _ = e.s.UpsertVariant(ctx, store.Variant{LinkID: e.l.ID, ExternalID: "b", Tracked: true})
	e.seedDays(t, 5, 27900)
	e.r.next = func(string) (reader.Listing, error) {
		return listing(vp("a", 27900, true), vp("b", 10000, false)), nil
	}
	e.check(t)
	if len(e.mail.subjects) != 0 {
		t.Fatalf("mail = %v", e.mail.subjects)
	}
}

func TestPausedSkippedByRunAll(t *testing.T) {
	e := setup(t)
	e.p.Paused = true
	_ = e.s.UpdateProduct(context.Background(), e.p)
	if err := e.c.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.r.calls != 0 {
		t.Fatalf("paused product was read %d times", e.r.calls)
	}
}

func TestPreviewCanonicalizes(t *testing.T) {
	e := setup(t)
	l, err := e.c.Preview(context.Background(), "https://fake.test/9?utm=1")
	if err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://fake.test/9" || len(l.Variants) != 1 {
		t.Fatalf("preview = %+v", l)
	}
}

func TestBadUntrackedVariantDoesNotBreakLink(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 25000, true), vp("b", 0, true)), nil }
	e.check(t)
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status != store.LinkOK {
		t.Fatalf("link = %+v", l)
	}
	if latest, _ := e.s.LatestReadings(context.Background(), e.p.ID); latest[e.v.ID].BestCents != 25000 {
		t.Fatalf("latest = %+v", latest)
	}
}

func TestBadTrackedVariantBreaksLink(t *testing.T) {
	e := setup(t)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 0, true), vp("b", 100, true)), nil }
	e.failUntilBroken(t)
	if l, _ := e.s.GetLink(context.Background(), e.l.ID); l.Status != store.LinkBroken {
		t.Fatalf("link = %+v", l)
	}
	if latest, _ := e.s.LatestReadings(context.Background(), e.p.ID); len(latest) != 0 {
		t.Fatalf("recorded a reading: %+v", latest)
	}
}

func TestGoneTrackedVariantStillRequested(t *testing.T) {
	e := setup(t)
	var wants [][]string
	reader.Register(&wantRecorder{fakeReader: e.r, wants: &wants})
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("z", 100, true)), nil }
	e.check(t) // "a" goes missing and is marked gone
	e.check(t)
	if len(wants) != 2 || len(wants[1]) != 1 || wants[1][0] != "a" {
		t.Fatalf("wants = %v", wants)
	}
}

type wantRecorder struct {
	*fakeReader
	wants *[][]string
}

func (w *wantRecorder) Read(ctx context.Context, b reader.Browser, url string, want []string) (reader.Listing, error) {
	*w.wants = append(*w.wants, want)
	return w.fakeReader.Read(ctx, b, url, want)
}

func TestBlockOnSiteWithoutLoginRetriesLater(t *testing.T) {
	e := setup(t)
	clock := now
	e.c.Now = func() time.Time { return clock }
	e.r.next = func(string) (reader.Listing, error) { return reader.Listing{}, reader.ErrBlocked }
	e.check(t)
	clock = clock.Add(time.Hour)
	e.check(t)
	if e.r.calls != 1 {
		t.Fatalf("retried after 1h: calls = %d", e.r.calls)
	}
	clock = clock.Add(6 * time.Hour)
	e.r.next = func(string) (reader.Listing, error) { return listing(vp("a", 25000, true)), nil }
	e.check(t)
	if e.r.calls != 2 {
		t.Fatalf("never retried: calls = %d", e.r.calls)
	}
	if st, _ := e.s.GetSession(context.Background(), "fake"); st != store.SessionOK {
		t.Fatalf("session = %q", st)
	}
}
