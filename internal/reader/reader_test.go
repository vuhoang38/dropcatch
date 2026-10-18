package reader

import (
	"context"
	"errors"
	"testing"
)

func good() Listing {
	return Listing{Site: Shopee, Title: "AirPods", Variants: []VariantPrice{{ExternalID: "1", PriceCents: 31600, BestCents: 31500, InStock: true}}}
}

func TestValidate(t *testing.T) {
	if err := Validate(good()); err != nil {
		t.Fatalf("good listing: %v", err)
	}
	bad := map[string]func(*Listing){
		"no variants":   func(l *Listing) { l.Variants = nil },
		"zero best":     func(l *Listing) { l.Variants[0].BestCents = 0 },
		"zero price":    func(l *Listing) { l.Variants[0].PriceCents = 0 },
		"best > price":  func(l *Listing) { l.Variants[0].BestCents = 40000 },
		"no externalID": func(l *Listing) { l.Variants[0].ExternalID = "" },
	}
	for name, mutate := range bad {
		l := good()
		mutate(&l)
		if err := Validate(l); !errors.Is(err, ErrUnreadable) {
			t.Errorf("%s: err = %v, want ErrUnreadable", name, err)
		}
	}
}

type stub struct{ site Site }

func (s stub) Site() Site { return s.site }
func (s stub) Canonical(raw string) (string, bool) {
	if raw == "https://stub.test/x?utm=1" {
		return "https://stub.test/x", true
	}
	return "", false
}
func (stub) Read(ctx context.Context, b Browser, url string, want []string) (Listing, error) {
	return Listing{}, nil
}

func TestFor(t *testing.T) {
	Register(stub{site: "stub"})
	r, canon, err := For("https://stub.test/x?utm=1")
	if err != nil || r.Site() != "stub" || canon != "https://stub.test/x" {
		t.Fatalf("For = %v %q %v", r, canon, err)
	}
	if _, _, err := For("https://example.com/x"); err == nil {
		t.Fatal("want unsupported site error")
	}
}
