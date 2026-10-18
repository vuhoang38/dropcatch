package scheduler

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dropcatch/internal/checker"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

var base = time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)

func TestDue(t *testing.T) {
	iv := 3 * time.Hour
	cases := []struct {
		last time.Time
		want bool
	}{
		{time.Time{}, true},
		{base.Add(-(2*time.Hour + 59*time.Minute)), false},
		{base.Add(-3 * time.Hour), true},
		{base.Add(-9 * time.Hour), true},
	}
	for _, c := range cases {
		if got := Due(c.last, iv, base); got != c.want {
			t.Errorf("Due(%v) = %v; want %v", c.last, got, c.want)
		}
	}
}

func TestStale(t *testing.T) {
	limit := 12 * time.Hour
	if Stale(base.Add(-(11*time.Hour + 59*time.Minute)), true, base, limit) {
		t.Error("11h59m should not be stale")
	}
	if !Stale(base.Add(-12*time.Hour), true, base, limit) {
		t.Error("12h should be stale")
	}
	if Stale(base.Add(-48*time.Hour), false, base, limit) {
		t.Error("nothing tracked should never be stale")
	}
}

type countingReader struct{ calls int }

func (r *countingReader) Site() reader.Site { return "sched" }
func (r *countingReader) Canonical(raw string) (string, bool) {
	return raw, strings.HasPrefix(raw, "https://sched.test/")
}
func (r *countingReader) Read(ctx context.Context, b reader.Browser, url string, want []string) (reader.Listing, error) {
	r.calls++
	return reader.Listing{Variants: []reader.VariantPrice{{ExternalID: "a", PriceCents: 100, BestCents: 100, InStock: true}}}, nil
}

type mailbox struct{ subjects []string }

func (m *mailbox) Send(ctx context.Context, to, subject, text string) error {
	m.subjects = append(m.subjects, subject)
	return nil
}

func setup(t *testing.T, clock *time.Time) (*Scheduler, *store.Store, *countingReader, *mailbox) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	p, _ := s.CreateProduct(ctx, "X")
	l, _ := s.CreateLink(ctx, p.ID, "https://sched.test/1", "sched", "", "")
	_, _ = s.UpsertVariant(ctx, store.Variant{LinkID: l.ID, ExternalID: "a", Tracked: true})

	r := &countingReader{}
	reader.Register(r)
	mail := &mailbox{}
	now := func() time.Time { return *clock }
	notices := &notify.Notices{Store: s, Mailer: mail, Now: now, To: func(context.Context) (string, error) { return "me@x.com", nil }}
	c := &checker.Checker{Store: s, Mailer: mail, Notices: notices, Now: now, Sleep: func(time.Duration) {}}
	return &Scheduler{Checker: c, Store: s, Notices: notices, Now: now}, s, r, mail
}

func TestTickOnceRunsOnceAfterWake(t *testing.T) {
	clock := base
	sch, s, r, _ := setup(t, &clock)
	_ = s.SetTime(context.Background(), "last_run_at", base.Add(-9*time.Hour))
	sch.TickOnce(context.Background())
	clock = clock.Add(time.Minute)
	sch.TickOnce(context.Background())
	if r.calls != 1 {
		t.Fatalf("reader calls = %d, want 1", r.calls)
	}
}

func TestStaleEmailOnceThenClears(t *testing.T) {
	clock := base
	sch, s, _, mail := setup(t, &clock)
	ctx := context.Background()
	// Recent run, so no check is due, but the last success was long ago.
	_ = s.SetTime(ctx, "last_run_at", base)
	_ = s.SetTime(ctx, "last_success_at", base.Add(-13*time.Hour))
	sch.TickOnce(ctx)
	sch.TickOnce(ctx)
	if len(mail.subjects) != 1 || !strings.Contains(mail.subjects[0], "12 hours") {
		t.Fatalf("mail = %v", mail.subjects)
	}
	_ = s.SetTime(ctx, "last_success_at", base)
	sch.TickOnce(ctx)
	if on, _ := s.NoticeActive(ctx, "stale"); on {
		t.Fatal("stale notice should clear")
	}
}

func TestFreshInstallNotStale(t *testing.T) {
	clock := base // product was created "now" by setup's real clock; use its CreatedAt
	sch, s, _, mail := setup(t, &clock)
	ctx := context.Background()
	p, _ := s.GetProduct(ctx, 1)
	clock = p.CreatedAt.Add(time.Hour)
	_ = s.SetTime(ctx, "last_run_at", clock)
	sch.TickOnce(ctx)
	if len(mail.subjects) != 0 {
		t.Fatalf("mail = %v", mail.subjects)
	}
}

type hangingReader struct{ countingReader }

func (r *hangingReader) Canonical(raw string) (string, bool) {
	return raw, strings.HasPrefix(raw, "https://hang.test/")
}
func (r *hangingReader) Site() reader.Site { return "hang" }
func (r *hangingReader) Read(ctx context.Context, b reader.Browser, url string, want []string) (reader.Listing, error) {
	<-ctx.Done() // a page that never finishes loading
	return reader.Listing{}, ctx.Err()
}

func TestStaleEmailWhileCheckHangs(t *testing.T) {
	clock := base
	sch, s, _, _ := setup(t, &clock)
	ctx := context.Background()
	_ = s.DeleteLink(ctx, 1)
	l, _ := s.CreateLink(ctx, 1, "https://hang.test/1", "hang", "", "")
	_, _ = s.UpsertVariant(ctx, store.Variant{LinkID: l.ID, ExternalID: "a", Tracked: true})
	reader.Register(&hangingReader{})
	// Not stale yet. last_run_at is unset, so the first tick starts a check
	// that never ends.
	_ = s.SetTime(ctx, "last_success_at", base.Add(-11*time.Hour))

	var now atomic.Int64
	now.Store(base.UnixNano())
	clockFn := func() time.Time { return time.Unix(0, now.Load()) }
	sch.Now, sch.Notices.Now, sch.Checker.Now = clockFn, clockFn, clockFn
	sch.Tick = 10 * time.Millisecond

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go sch.Run(runCtx)
	time.Sleep(50 * time.Millisecond) // the check is hanging now
	now.Store(base.Add(2 * time.Hour).UnixNano())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if on, _ := s.NoticeActive(ctx, "stale"); on {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("stale email never sent while a check hung")
}
