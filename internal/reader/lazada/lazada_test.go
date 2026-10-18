package lazada

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"dropcatch/internal/reader"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseIDs(t *testing.T) {
	cases := map[string][2]string{
		"https://www.lazada.sg/products/airpods-pro-3-i3525361975-s23252620260.html?spm=x": {"3525361975", "23252620260"},
		"https://www.lazada.sg/products/pdp-i3525361975.html":                              {"3525361975", ""},
		"//www.lazada.sg/products/pdp-i13983528912.html":                                   {"13983528912", ""},
	}
	for raw, want := range cases {
		item, sku, ok := ParseIDs(raw)
		if !ok || item != want[0] || sku != want[1] {
			t.Errorf("ParseIDs(%q) = %q %q %v", raw, item, sku, ok)
		}
	}
	if _, _, ok := ParseIDs("https://www.lazada.sg/catalog/?q=x"); ok {
		t.Error("search page should not parse")
	}
}

func TestCanonical(t *testing.T) {
	got, ok := New().Canonical("https://www.lazada.sg/products/airpods-pro-3-i3525361975-s23252620260.html?spm=x")
	if !ok || got != "https://www.lazada.sg/products/pdp-i3525361975.html" {
		t.Fatalf("canonical = %q %v", got, ok)
	}
}

func TestParseAirpods(t *testing.T) {
	l, err := ParseDetail(load(t, "airpods_logged_in.json"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "AirPods Pro 3" || l.Seller != "Apple Flagship Store" || len(l.Variants) != 1 {
		t.Fatalf("listing = %+v", l)
	}
	want := reader.VariantPrice{ExternalID: "23252620260", PriceCents: 32200, BestCents: 32200, OriginalCents: 34900, InStock: true}
	if l.Variants[0] != want {
		t.Fatalf("variant = %+v", l.Variants[0])
	}
}

func TestParseLoggedOut(t *testing.T) {
	if _, err := ParseDetail(load(t, "airpods_logged_out.json")); !errors.Is(err, reader.ErrLoggedOut) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseMultiVariant(t *testing.T) {
	l, err := ParseDetail(load(t, "case_multi.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Variants) != 36 {
		t.Fatalf("variants = %d", len(l.Variants))
	}
	byID := map[string]reader.VariantPrice{}
	for _, v := range l.Variants {
		byID[v.ExternalID] = v
	}
	if v := byID["23528608337"]; v.PriceCents != 1194 || v.InStock {
		t.Errorf("wishlist-only variant = %+v", v)
	}
	if v := byID["23528608336"]; v.PriceCents != 1195 || !v.InStock {
		t.Errorf("in stock variant = %+v", v)
	}
	if v := byID["23528608335"]; !strings.HasPrefix(v.Name, "[gray]") || !strings.Contains(v.Name, " / ") {
		t.Errorf("name = %q", v.Name)
	}
}

type fakeBrowser struct {
	final string
	body  []byte
	urls  []string
	loads []map[string][]byte // responses for each Load in turn, instead of body
}

func (f *fakeBrowser) Load(ctx context.Context, url string, capture []string) (reader.Page, error) {
	f.urls = append(f.urls, url)
	p := reader.Page{FinalURL: f.final, Responses: map[string][]byte{}}
	if p.FinalURL == "" {
		p.FinalURL = url
	}
	if f.body != nil {
		p.Responses["https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?x"] = f.body
	}
	if n := len(f.urls) - 1; n < len(f.loads) {
		p.Responses = f.loads[n]
	}
	return p, nil
}

const tokenExpired = `{"ret":["FAIL_SYS_TOKEN_EXOIRED::令牌过期"],"data":{}}`

// Lazada's page script gets a new token and asks again, so a page can carry
// a failed reply next to the good one.
func TestReadSkipsExpiredTokenReply(t *testing.T) {
	good := load(t, "airpods_logged_in.json")
	for range 20 { // map order is random; make sure the failed reply never wins
		r := &Reader{Sleep: func(time.Duration) {}}
		b := &fakeBrowser{loads: []map[string][]byte{{
			"https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?t=1": []byte(tokenExpired),
			"https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?t=2": good,
		}}}
		l, err := r.Read(context.Background(), b, "https://www.lazada.sg/products/pdp-i3525361975.html", nil)
		if err != nil || l.Variants[0].PriceCents != 32200 || len(b.urls) != 1 {
			t.Fatalf("err = %v urls = %v listing = %+v", err, b.urls, l)
		}
	}
}

// When the retry doesn't land before the page settles, the token cookie is
// fresh by then, so one more load works.
func TestReadReloadsAfterExpiredToken(t *testing.T) {
	r := &Reader{Sleep: func(time.Duration) {}}
	b := &fakeBrowser{loads: []map[string][]byte{
		{"https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?t=1": []byte(tokenExpired)},
		{"https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?t=2": load(t, "airpods_logged_in.json")},
	}}
	l, err := r.Read(context.Background(), b, "https://www.lazada.sg/products/pdp-i3525361975.html", nil)
	if err != nil || l.Variants[0].PriceCents != 32200 || len(b.urls) != 2 {
		t.Fatalf("err = %v urls = %v listing = %+v", err, b.urls, l)
	}
}

func TestReadGivesUpAfterTwoExpiredTokens(t *testing.T) {
	r := &Reader{Sleep: func(time.Duration) {}}
	expired := map[string][]byte{"https://acs-m.lazada.sg/h5/mtop.global.detail.web.getdetailinfo/1.0/?t=1": []byte(tokenExpired)}
	b := &fakeBrowser{loads: []map[string][]byte{expired, expired, expired}}
	if _, err := r.Read(context.Background(), b, "https://www.lazada.sg/products/pdp-i1.html", nil); !errors.Is(err, reader.ErrUnreadable) || len(b.urls) != 2 {
		t.Fatalf("err = %v urls = %v", err, b.urls)
	}
}

func TestReadBlocked(t *testing.T) {
	r := &Reader{Sleep: func(time.Duration) {}}
	b := &fakeBrowser{final: "https://www.lazada.sg//products/pdp-i1.html/_____tmd_____/punish?x5secdata=abc"}
	if _, err := r.Read(context.Background(), b, "https://www.lazada.sg/products/pdp-i1.html", nil); !errors.Is(err, reader.ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadMissingResponse(t *testing.T) {
	r := &Reader{Sleep: func(time.Duration) {}}
	if _, err := r.Read(context.Background(), &fakeBrowser{}, "https://www.lazada.sg/products/pdp-i1.html", nil); !errors.Is(err, reader.ErrUnreadable) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadKeepsOnlyLoadedPrices(t *testing.T) {
	r := &Reader{Sleep: func(time.Duration) {}}
	b := &fakeBrowser{body: load(t, "airpods_logged_in.json")}
	l, err := r.Read(context.Background(), b, "https://www.lazada.sg/products/pdp-i3525361975.html", []string{"23252620260"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.urls) != 1 || l.URL != "https://www.lazada.sg/products/pdp-i3525361975.html" || l.Variants[0].PriceCents != 32200 {
		t.Fatalf("urls = %v listing = %+v", b.urls, l)
	}
}

func TestParseRetCodes(t *testing.T) {
	cases := map[string]error{
		`{"ret":["RGV587_ERROR::SM::哎哟喂,被挤爆啦,请稍后重试"],"data":{"url":"https://www.lazada.sg//_____tmd_____/punish"}}`: reader.ErrBlocked,
		`{"ret":["FAIL_SYS_USER_VALIDATE::验证"],"data":{}}`:                                                          reader.ErrBlocked,
		`{"ret":["FAIL_SYS_SESSION_EXPIRED::Session过期"],"data":{}}`:                                                 reader.ErrLoggedOut,
		`{"ret":["FAIL_SYS_ILLEGAL_ACCESS::x"],"data":{}}`:                                                          reader.ErrUnreadable,
	}
	for body, want := range cases {
		if _, err := ParseDetail([]byte(body)); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", body[:30], err, want)
		}
	}
}
