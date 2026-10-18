package courts

import (
	"errors"
	"os"
	"testing"

	"dropcatch/internal/reader"
)

func TestCanonical(t *testing.T) {
	r := New()
	got, ok := r.Canonical("https://www.courts.com.sg/apple-airpods-pro-3-mfhp4za%2Fa-IP214198.html?cgid=x#top")
	if !ok || got != "https://www.courts.com.sg/apple-airpods-pro-3-mfhp4za%2Fa-IP214198.html" {
		t.Fatalf("canonical = %q %v", got, ok)
	}
	if _, ok := r.Canonical("https://www.courts.com.sg/search?q=airpods"); ok {
		t.Fatal("search page is not a product")
	}
}

func TestParseHTML(t *testing.T) {
	b, err := os.ReadFile("testdata/airpods_pro3.html")
	if err != nil {
		t.Fatal(err)
	}
	l, err := ParseHTML(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "APPLE Airpods Pro 3 MFHP4ZA/A" || l.Seller != "COURTS" || len(l.Variants) != 1 {
		t.Fatalf("listing = %+v", l)
	}
	want := reader.VariantPrice{ExternalID: "IP214198", PriceCents: 27900, BestCents: 27900, OriginalCents: 34900, InStock: false}
	if l.Variants[0] != want {
		t.Fatalf("variant = %+v", l.Variants[0])
	}
}

func TestParseHTMLNoProduct(t *testing.T) {
	if _, err := ParseHTML(`<html><title>404</title></html>`); !errors.Is(err, reader.ErrUnreadable) {
		t.Fatalf("err = %v", err)
	}
}
