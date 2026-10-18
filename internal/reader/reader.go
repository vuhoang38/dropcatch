// Package reader defines how a site's product page becomes prices. Each site
// lives in its own subpackage and registers itself here.
package reader

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Site string

const (
	Shopee Site = "shopee"
	Lazada Site = "lazada"
	Amazon Site = "amazon"
	Courts Site = "courts"
)

type VariantPrice struct {
	ExternalID string
	Name       string
	PriceCents int64
	BestCents  int64 // after vouchers or coupons shown on the page
	// The seller's "was" price, 0 if none. Display only, never used for alerts.
	OriginalCents int64
	// True when the page didn't show a voucher price for this variant, so
	// BestCents is just the plain price.
	BestIsEstimate bool
	InStock        bool
}

type Listing struct {
	Site     Site
	URL      string
	Title    string
	Seller   string
	Variants []VariantPrice
}

var (
	ErrLoggedOut  = errors.New("logged out")
	ErrBlocked    = errors.New("blocked by verification")
	ErrUnreadable = errors.New("price not found")
)

// Page is what a browser saw after loading a URL.
type Page struct {
	FinalURL  string
	HTML      string
	Responses map[string][]byte // body by response URL
}

type Browser interface {
	// Load opens url and returns the page plus the bodies of responses whose
	// URL contains any of the capture substrings.
	Load(ctx context.Context, url string, capture []string) (Page, error)
}

type Reader interface {
	Site() Site
	// Canonical returns the one URL to store for this listing, and false when
	// this reader doesn't handle raw.
	Canonical(raw string) (string, bool)
	// Read prices the listing. want lists variant external IDs to price
	// exactly. Empty means whatever the page shows by default.
	Read(ctx context.Context, b Browser, url string, want []string) (Listing, error)
}

var (
	mu      sync.RWMutex
	readers []Reader
)

// Register adds a reader. A reader for a site already registered replaces it.
func Register(r Reader) {
	mu.Lock()
	defer mu.Unlock()
	for i, have := range readers {
		if have.Site() == r.Site() {
			readers[i] = r
			return
		}
	}
	readers = append(readers, r)
}

// For finds the reader for raw and returns it with the canonical URL.
func For(raw string) (Reader, string, error) {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range readers {
		if canon, ok := r.Canonical(raw); ok {
			return r, canon, nil
		}
	}
	return nil, "", fmt.Errorf("unsupported site: %s", raw)
}

// Validate rejects listings that would record a wrong price.
func Validate(l Listing) error {
	if len(l.Variants) == 0 {
		return fmt.Errorf("%w: no variants on page", ErrUnreadable)
	}
	for _, v := range l.Variants {
		switch {
		case v.ExternalID == "":
			return fmt.Errorf("%w: variant without id", ErrUnreadable)
		case v.PriceCents <= 0 || v.BestCents <= 0:
			return fmt.Errorf("%w: zero price for %q", ErrUnreadable, v.Name)
		case v.BestCents > v.PriceCents:
			return fmt.Errorf("%w: best price above price for %q", ErrUnreadable, v.Name)
		}
	}
	return nil
}
