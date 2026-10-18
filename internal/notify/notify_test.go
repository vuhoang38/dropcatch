package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dropcatch/internal/store"
)

func TestResendRequest(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	var auth, path, method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path, method = r.Header.Get("Authorization"), r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	m := &Resend{APIKey: "k", BaseURL: srv.URL}
	if err := m.Send(context.Background(), "me@x.com", "Hi", "Body"); err != nil {
		t.Fatal(err)
	}
	if method != "POST" || path != "/emails" || auth != "Bearer k" {
		t.Fatalf("request = %s %s %q", method, path, auth)
	}
	if got.From != "Dropcatch <onboarding@resend.dev>" || len(got.To) != 1 || got.To[0] != "me@x.com" || got.Subject != "Hi" || got.Text != "Body" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestResendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		io.WriteString(w, `{"message":"You can only send testing emails to your own email address"}`)
	}))
	defer srv.Close()
	err := (&Resend{APIKey: "k", BaseURL: srv.URL}).Send(context.Background(), "a@b.c", "s", "t")
	if err == nil || !strings.Contains(err.Error(), "only send testing emails") {
		t.Fatalf("err = %v", err)
	}
}

func TestResendNeedsKey(t *testing.T) {
	if err := (&Resend{}).Send(context.Background(), "a@b.c", "s", "t"); err == nil {
		t.Fatal("want error without API key")
	}
}

func TestDropEmail(t *testing.T) {
	subject, text := DropEmail(DropAlert{
		Product: "AirPods Pro 3", Site: "shopee", Seller: "Apple Flagship Store", Variant: "Pro 3",
		URL: "https://shopee.sg/product/1/2", NewCents: 22900, TypicalCents: 27900, DropPct: 17.92,
		HistoryDays: 3, WindowDays: 30,
	})
	if subject != "AirPods Pro 3 dropped to $229.00 (18% under typical)" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{"$279.00", "https://shopee.sg/product/1/2", "Based on 3 days of history.", "Shopee · Apple Flagship Store · Pro 3", "last 30 days"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}
	_, full := DropEmail(DropAlert{Product: "X", NewCents: 1, TypicalCents: 2, HistoryDays: 30, WindowDays: 30})
	if strings.Contains(full, "Based on") {
		t.Error("full history should not get the thin history note")
	}
}

type recorder struct{ sent []string }

func (r *recorder) Send(ctx context.Context, to, subject, text string) error {
	r.sent = append(r.sent, subject)
	return nil
}

func TestNoticesOnce(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rec := &recorder{}
	n := &Notices{Store: s, Mailer: rec, To: func(context.Context) (string, error) { return "me@x.com", nil }, Now: time.Now}
	ctx := context.Background()
	for range 2 {
		if err := n.Raise(ctx, "link:1", "broken", "text"); err != nil {
			t.Fatal(err)
		}
	}
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d, want 1", len(rec.sent))
	}
	_ = n.Clear(ctx, "link:1")
	_ = n.Raise(ctx, "link:1", "broken", "text")
	if len(rec.sent) != 2 {
		t.Fatalf("sent %d, want 2", len(rec.sent))
	}
}

func TestSettingsMailerUsesStoredKey(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
	}))
	defer srv.Close()
	s, _ := store.Open(":memory:")
	defer s.Close()
	set := store.DefaultSettings()
	set.ResendAPIKey = "re_123"
	_ = s.PutSettings(context.Background(), set)
	m := &SettingsMailer{Store: s, BaseURL: srv.URL}
	if err := m.Send(context.Background(), "me@x.com", "s", "t"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_123" {
		t.Fatalf("auth = %q", auth)
	}
}
