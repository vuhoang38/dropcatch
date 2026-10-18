// Package courts reads prices from courts.com.sg product pages.
//
// The page carries a schema.org Product in JSON-LD with the sale price and
// stock. The "RRP" strike-through price sits in .old-value-price[content].
// Courts has no variants and no vouchers on the page.
package courts

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"dropcatch/internal/money"
	"dropcatch/internal/reader"
)

type Reader struct{}

func New() *Reader { return &Reader{} }

func (r *Reader) Site() reader.Site { return reader.Courts }

var pidRe = regexp.MustCompile(`(?i)-(IP\d+)\.html$`)

func (r *Reader) Canonical(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Host != "www.courts.com.sg" && u.Host != "courts.com.sg") {
		return "", false
	}
	if !pidRe.MatchString(u.Path) {
		return "", false
	}
	return "https://www.courts.com.sg" + u.EscapedPath(), true
}

func (r *Reader) Read(ctx context.Context, b reader.Browser, rawURL string, want []string) (reader.Listing, error) {
	page, err := b.Load(ctx, rawURL, nil)
	if err != nil {
		return reader.Listing{}, err
	}
	l, err := ParseHTML(page.HTML)
	if err != nil {
		return l, err
	}
	l.URL = rawURL
	return l, reader.Validate(l)
}

var (
	pdpPidRe   = regexp.MustCompile(`class="container product-detail[^"]*"\s+data-pid="([^"]+)"`)
	oldPriceRe = regexp.MustCompile(`class="value old-value-price" content="([\d.]+)"`)
)

func ParseHTML(html string) (reader.Listing, error) {
	name, price, inStock, ok := reader.ProductFromJSONLD(html)
	m := pdpPidRe.FindStringSubmatch(html)
	if !ok || m == nil {
		return reader.Listing{}, fmt.Errorf("%w: no product data on page", reader.ErrUnreadable)
	}
	v := reader.VariantPrice{ExternalID: m[1], PriceCents: price, BestCents: price, InStock: inStock}
	if om := oldPriceRe.FindStringSubmatch(html); om != nil {
		if orig, err := money.ParseSGD(om[1]); err == nil && orig > price {
			v.OriginalCents = orig
		}
	}
	return reader.Listing{
		Site:     reader.Courts,
		Title:    strings.Join(strings.Fields(name), " "),
		Seller:   "COURTS",
		Variants: []reader.VariantPrice{v},
	}, nil
}
