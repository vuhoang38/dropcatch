package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"dropcatch/internal/alert"
	"dropcatch/internal/checker"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

type fakeReader struct{}

func (fakeReader) Site() reader.Site { return "fake" }
func (fakeReader) Canonical(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "https://fake.test/") {
		return "", false
	}
	return strings.SplitN(raw, "?", 2)[0], true
}
func (fakeReader) Read(ctx context.Context, b reader.Browser, url string, want []string) (reader.Listing, error) {
	return reader.Listing{Title: "AirPods Pro 3", Seller: "Apple", Variants: []reader.VariantPrice{
		{ExternalID: "a", Name: "USB-C", PriceCents: 31600, BestCents: 31500, InStock: true},
		{ExternalID: "b", Name: "Case only", PriceCents: 9900, BestCents: 9900, InStock: true},
	}}, nil
}

type mailbox struct{ subjects []string }

func (m *mailbox) Send(ctx context.Context, to, subject, text string) error {
	m.subjects = append(m.subjects, subject)
	return nil
}

var now = time.Date(2026, 10, 18, 10, 0, 0, 0, alert.SGT)

func setup(t *testing.T) (http.Handler, *store.Store, *mailbox) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	reader.Register(fakeReader{})
	mail := &mailbox{}
	nowFn := func() time.Time { return now }
	c := &checker.Checker{Store: s, Mailer: mail, Now: nowFn, Sleep: func(time.Duration) {}}
	static := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "assets/app.js": {Data: []byte("js")}}
	return New(Deps{Store: s, Checker: c, Mailer: mail, Now: nowFn}, static), s, mail
}

func do(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func decode[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(bytes.NewBufferString(s)).Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func createAirpods(t *testing.T, h http.Handler) ProductDetail {
	t.Helper()
	code, body := do(t, h, "POST", "/api/products", `{"name":"AirPods Pro 3","links":[{"url":"https://fake.test/1?utm=x","variantIds":["a"]}]}`)
	if code != 201 {
		t.Fatalf("create = %d %s", code, body)
	}
	return decode[ProductDetail](t, body)
}

func TestCreateProductTracksChosenVariants(t *testing.T) {
	h, _, _ := setup(t)
	p := createAirpods(t, h)
	if p.Name != "AirPods Pro 3" || len(p.Links) != 1 {
		t.Fatalf("product = %+v", p)
	}
	l := p.Links[0]
	if l.URL != "https://fake.test/1" || l.Title != "AirPods Pro 3" || len(l.Variants) != 2 {
		t.Fatalf("link = %+v", l)
	}
	if !l.Variants[0].Tracked || l.Variants[1].Tracked || l.Variants[0].IsNew {
		t.Fatalf("variants = %+v", l.Variants)
	}
	if p.Settings.DropPct != 20 || p.Overrides.DropPct != nil {
		t.Fatalf("settings = %+v overrides = %+v", p.Settings, p.Overrides)
	}
}

func TestDuplicateLinkConflict(t *testing.T) {
	h, _, _ := setup(t)
	p := createAirpods(t, h)
	code, body := do(t, h, "POST", "/api/products/1/links", `{"url":"https://fake.test/1?other=param","variantIds":["a"]}`)
	if code != 409 || !strings.Contains(body, "This link is already tracked") {
		t.Fatalf("dup = %d %s (product %d)", code, body, p.ID)
	}
}

func TestPatchOverrides(t *testing.T) {
	h, _, _ := setup(t)
	createAirpods(t, h)
	code, body := do(t, h, "PATCH", "/api/products/1", `{"dropPct":15,"paused":true}`)
	p := decode[ProductDetail](t, body)
	if code != 200 || p.Overrides.DropPct == nil || *p.Overrides.DropPct != 15 || p.Settings.DropPct != 15 || !p.Paused {
		t.Fatalf("patch = %d %+v", code, p)
	}
	_, body = do(t, h, "PATCH", "/api/products/1", `{"dropPct":null}`)
	p = decode[ProductDetail](t, body)
	if p.Overrides.DropPct != nil || p.Settings.DropPct != 20 || !p.Paused {
		t.Fatalf("clear = %+v", p)
	}
}

func TestSummaryBestAndTypical(t *testing.T) {
	h, s, _ := setup(t)
	createAirpods(t, h)
	ctx := context.Background()
	v, _ := s.GetVariant(ctx, 1)
	for ago := 5; ago >= 1; ago-- {
		_ = s.AddReading(ctx, store.Reading{VariantID: v.ID, At: now.AddDate(0, 0, -ago), PriceCents: 27900, BestCents: 27900, InStock: true})
	}
	_ = s.AddReading(ctx, store.Reading{VariantID: v.ID, At: now, PriceCents: 22900, BestCents: 22900, InStock: true})
	code, body := do(t, h, "GET", "/api/products", "")
	list := decode[[]ProductSummary](t, body)
	if code != 200 || len(list) != 1 {
		t.Fatalf("list = %d %s", code, body)
	}
	got := list[0]
	if got.BestCents == nil || *got.BestCents != 22900 || got.BestSite != "fake" || got.TypicalCents == nil || *got.TypicalCents != 27900 {
		t.Fatalf("summary = %+v", got)
	}
	if got.PctVsTypical == nil || *got.PctVsTypical > -17.9 || *got.PctVsTypical < -18 {
		t.Fatalf("pct = %v", got.PctVsTypical)
	}
	if len(got.Spark) != 6 {
		t.Fatalf("spark = %+v", got.Spark)
	}
}

func TestHistory(t *testing.T) {
	h, s, _ := setup(t)
	createAirpods(t, h)
	ctx := context.Background()
	_ = s.AddReading(ctx, store.Reading{VariantID: 1, At: now.AddDate(0, 0, -40), PriceCents: 30000, BestCents: 30000, InStock: true})
	_ = s.AddReading(ctx, store.Reading{VariantID: 1, At: now.AddDate(0, 0, -1), PriceCents: 27900, BestCents: 27900, InStock: true})
	_, body := do(t, h, "GET", "/api/products/1/history?range=30d", "")
	hist := decode[History](t, body)
	if len(hist.Series) != 1 || len(hist.Series[0].Points) != 1 || hist.Series[0].Label != "fake · Apple · USB-C" {
		t.Fatalf("history = %+v", hist)
	}
	_, body = do(t, h, "GET", "/api/products/1/history?range=all", "")
	if hist := decode[History](t, body); len(hist.Series[0].Points) != 2 {
		t.Fatalf("all = %+v", hist)
	}
}

func TestVariantTracking(t *testing.T) {
	h, s, _ := setup(t)
	createAirpods(t, h)
	if code, _ := do(t, h, "PATCH", "/api/variants/2", `{"tracked":true}`); code != 204 {
		t.Fatalf("patch = %d", code)
	}
	if v, _ := s.GetVariant(context.Background(), 2); !v.Tracked {
		t.Fatal("variant not tracked")
	}
}

func TestSettingsMaskKey(t *testing.T) {
	h, s, _ := setup(t)
	code, _ := do(t, h, "PUT", "/api/settings", `{"dropPct":20,"windowDays":30,"minHistoryDays":2,"checkIntervalHours":3,"resendApiKey":"re_secret1234","email":"me@x.com"}`)
	if code != 200 {
		t.Fatalf("put = %d", code)
	}
	_, body := do(t, h, "GET", "/api/settings", "")
	got := decode[store.Settings](t, body)
	if strings.Contains(body, "secret") || !strings.HasSuffix(got.ResendAPIKey, "1234") {
		t.Fatalf("get = %s", body)
	}
	got.DropPct = 25
	b, _ := json.Marshal(got)
	do(t, h, "PUT", "/api/settings", string(b))
	stored, _ := s.GetSettings(context.Background())
	if stored.ResendAPIKey != "re_secret1234" || stored.DropPct != 25 {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestSettingsValidation(t *testing.T) {
	h, _, _ := setup(t)
	code, body := do(t, h, "PUT", "/api/settings", `{"dropPct":0,"windowDays":30,"minHistoryDays":2,"checkIntervalHours":3}`)
	if code != 400 || !strings.Contains(body, "message") {
		t.Fatalf("put = %d %s", code, body)
	}
}

func TestTestEmail(t *testing.T) {
	h, _, mail := setup(t)
	if code, _ := do(t, h, "POST", "/api/settings/test-email", ""); code != 204 || len(mail.subjects) != 1 {
		t.Fatalf("test email = %d %v", code, mail.subjects)
	}
}

func TestSessions(t *testing.T) {
	h, s, _ := setup(t)
	_ = s.SetSession(context.Background(), "shopee", store.SessionExpired)
	_, body := do(t, h, "GET", "/api/sessions", "")
	got := decode[[]Session](t, body)
	if len(got) != 2 || got[0].Site != "shopee" || got[0].Status != "expired" || got[1].Status != "unknown" {
		t.Fatalf("sessions = %s", body)
	}
}

func TestStaticFallback(t *testing.T) {
	h, _, _ := setup(t)
	if code, body := do(t, h, "GET", "/products/1", ""); code != 200 || body != "<html>app</html>" {
		t.Fatalf("fallback = %d %q", code, body)
	}
	if code, body := do(t, h, "GET", "/assets/app.js", ""); code != 200 || body != "js" {
		t.Fatalf("asset = %d %q", code, body)
	}
	if code, _ := do(t, h, "GET", "/api/nope", ""); code != 404 {
		t.Fatalf("api 404 = %d", code)
	}
}

func TestNotFound(t *testing.T) {
	h, _, _ := setup(t)
	if code, body := do(t, h, "GET", "/api/products/99", ""); code != 404 || !strings.Contains(body, "message") {
		t.Fatalf("missing = %d %s", code, body)
	}
}

func TestLoginDone(t *testing.T) {
	cases := []struct {
		site    string
		cookies map[string]string
		want    bool
	}{
		{"shopee", map[string]string{}, false},
		{"shopee", map[string]string{"SPC_U": "-"}, false},
		{"shopee", map[string]string{"SPC_U": ""}, false},
		{"shopee", map[string]string{"SPC_U": "12345678901"}, true},
		{"lazada", map[string]string{"lzd_sid": "abc"}, false}, // a session alone isn't a login
		{"lazada", map[string]string{"lzd_uid": "1234567890"}, true},
		{"lazada", map[string]string{"SPC_U": "12345678901"}, false},
		{"amazon", map[string]string{"SPC_U": "1"}, false},
	}
	for _, c := range cases {
		if got := loginDone(c.site)(c.cookies); got != c.want {
			t.Errorf("loginDone(%s)(%v) = %v, want %v", c.site, c.cookies, got, c.want)
		}
	}
}

type fakeLogin struct {
	url  string
	done func(map[string]string) bool
	out  chan struct{}
}

func (f *fakeLogin) OpenVisible(ctx context.Context, url string, done func(map[string]string) bool, timeout time.Duration) error {
	f.url, f.done = url, done
	close(f.out)
	return nil
}

func TestLoginMarksSessionUnknownAndPassesDone(t *testing.T) {
	_, s, _ := setup(t)
	login := &fakeLogin{out: make(chan struct{})}
	h := New(Deps{Store: s, Login: login, Now: func() time.Time { return now }}, fstest.MapFS{})
	_ = s.SetSession(context.Background(), "shopee", store.SessionExpired)
	if code, _ := do(t, h, "POST", "/api/sessions/shopee/login", ""); code != 202 {
		t.Fatalf("login = %d", code)
	}
	<-login.out
	if login.done == nil || !login.done(map[string]string{"SPC_U": "12345678901"}) {
		t.Fatal("login window got no way to notice a finished login")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if st, _ := s.GetSession(context.Background(), "shopee"); st == store.SessionUnknown {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("session not reset after the login window closed")
}

type loginThatSucceeds struct{ out chan struct{} }

func (l *loginThatSucceeds) OpenVisible(ctx context.Context, url string, done func(map[string]string) bool, timeout time.Duration) error {
	defer close(l.out)
	done(map[string]string{"SPC_U": "12345678901"}) // Shopee set your user id
	return nil
}

func TestLoginSuccessMarksSessionOK(t *testing.T) {
	_, s, _ := setup(t)
	login := &loginThatSucceeds{out: make(chan struct{})}
	h := New(Deps{Store: s, Login: login, Now: func() time.Time { return now }}, fstest.MapFS{})
	_ = s.SetSession(context.Background(), "shopee", store.SessionExpired)
	do(t, h, "POST", "/api/sessions/shopee/login", "")
	<-login.out
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if st, _ := s.GetSession(context.Background(), "shopee"); st == store.SessionOK {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := s.GetSession(context.Background(), "shopee")
	t.Fatalf("session = %q, want ok", st)
}
