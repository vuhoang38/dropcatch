// Package amazon reads prices from amazon.sg product pages.
//
// One URL is one variant: Amazon gives each colour or size its own ASIN.
// The best price is the price minus any "Apply S$X coupon" or "Apply X%
// coupon" shown on the page. No login is needed.
package amazon

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"dropcatch/internal/money"
	"dropcatch/internal/reader"
)

type Reader struct{}

func New() *Reader { return &Reader{} }

func (r *Reader) Site() reader.Site { return reader.Amazon }

var asinRe = regexp.MustCompile(`/(?:dp|gp/product)/([A-Z0-9]{10})`)

func (r *Reader) Canonical(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Host != "www.amazon.sg" && u.Host != "amazon.sg") {
		return "", false
	}
	m := asinRe.FindStringSubmatch(u.Path)
	if m == nil {
		return "", false
	}
	return "https://www.amazon.sg/dp/" + m[1], true
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

var couponRe = regexp.MustCompile(`(?i)apply\s+(?:s?\$\s?([\d,]+(?:\.\d+)?)|(\d+(?:\.\d+)?)\s?%)\s+(?:coupon|voucher)`)

// Coupon returns the cents a coupon text takes off price, 0 if none.
func Coupon(text string, priceCents int64) int64 {
	m := couponRe.FindStringSubmatch(text)
	switch {
	case m == nil:
		return 0
	case m[1] != "":
		off, err := money.ParseSGD(m[1])
		if err != nil || off >= priceCents {
			return 0
		}
		return off
	default:
		pct, err := strconv.ParseFloat(m[2], 64)
		if err != nil || pct <= 0 || pct >= 100 {
			return 0
		}
		return int64(float64(priceCents)*pct/100 + 0.5)
	}
}

func ParseHTML(html string) (reader.Listing, error) {
	if strings.Contains(html, "Enter the characters you see below") || strings.Contains(html, "/errors/validateCaptcha") {
		return reader.Listing{}, reader.ErrBlocked
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return reader.Listing{}, fmt.Errorf("%w: %v", reader.ErrUnreadable, err)
	}
	text := func(sel string) string { return strings.TrimSpace(doc.Find(sel).First().Text()) }

	asin, _ := doc.Find(`input#ASIN, input[name="ASIN"]`).First().Attr("value")
	price := firstPrice(doc, "#corePrice_feature_div .a-offscreen", "#corePriceDisplay_desktop_feature_div .priceToPay .a-offscreen", "#corePriceDisplay_desktop_feature_div .a-offscreen")
	if asin == "" || price == 0 {
		return reader.Listing{}, fmt.Errorf("%w: no price on page", reader.ErrUnreadable)
	}
	orig := firstPrice(doc, "#corePriceDisplay_desktop_feature_div .basisPrice .a-offscreen", "#corePrice_desktop .a-text-price .a-offscreen")
	if orig <= price {
		orig = 0
	}

	var promo strings.Builder
	doc.Find(`#promoPriceBlockMessage_feature_div, [id^="couponText"], [id^="couponBadge"]`).Each(func(_ int, s *goquery.Selection) {
		promo.WriteString(s.Text())
		promo.WriteString(" ")
	})

	l := reader.Listing{
		Site:   reader.Amazon,
		Title:  strings.Join(strings.Fields(text("#productTitle")), " "),
		Seller: text(`#merchantInfoFeature_feature_div .offer-display-feature-text-message, .offer-display-feature-text-message`),
		Variants: []reader.VariantPrice{{
			ExternalID:    asin,
			PriceCents:    price,
			BestCents:     price - Coupon(promo.String(), price),
			OriginalCents: orig,
			InStock:       strings.Contains(strings.ToLower(text("#availability")), "in stock"),
		}},
	}
	return l, nil
}

func firstPrice(doc *goquery.Document, selectors ...string) int64 {
	for _, sel := range selectors {
		var found int64
		doc.Find(sel).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if c, err := money.ParseSGD(s.Text()); err == nil {
				found = c
				return false
			}
			return true
		})
		if found > 0 {
			return found
		}
	}
	return 0
}
