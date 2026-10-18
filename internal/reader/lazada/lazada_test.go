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
	return p, nil
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
