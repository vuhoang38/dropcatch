// Package shopee reads prices from shopee.sg.
//
// The product page calls /api/v4/pdp/get_pc, and the reader listens for that
// response rather than parsing page text. Money values there are integers in
// units of 1/100000 SGD.
//
// The "After Voucher" price (product_price.price.single_value) applies only to
// the model named in product_price.final_price_info.model_id. Checked by hand
// on 2026-10-18 against a 100 variant listing:
//   - With no extraParams, a multi variant listing shows a price range and no
//     final_price_info.model_id at all.
//   - ?extraParams={"display_model_id":<id>} alone does not select the model.
//   - Adding "model_selection_logic":3 does. final_price_info.model_id then
//     names that model and single_value is its After Voucher price.
//   - A model with no applicable voucher still comes back without a final
//     model, so it falls back to its plain price, marked as an estimate.
//
// So Read loads once per tracked model with both params.
package shopee

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dropcatch/internal/reader"
)

const apiPath = "/api/v4/pdp/get_pc"

type Reader struct {
	// Sleep pauses between page loads. Nil means a random 5 to 20 seconds.
	Sleep func(time.Duration)
}

func New() *Reader { return &Reader{} }

func (r *Reader) Site() reader.Site { return reader.Shopee }

var (
	slugRe    = regexp.MustCompile(`-i\.(\d+)\.(\d+)`)
	productRe = regexp.MustCompile(`/product/(\d+)/(\d+)`)
)

// ParseIDs pulls the shop and item ids out of any shopee.sg product URL shape.
func ParseIDs(raw string) (shopID, itemID int64, ok bool) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || !isShopeeHost(u.Host) {
		return 0, 0, false
	}
	for _, re := range []*regexp.Regexp{slugRe, productRe} {
		if m := re.FindStringSubmatch(u.Path); m != nil {
			shop, _ := strconv.ParseInt(m[1], 10, 64)
			item, _ := strconv.ParseInt(m[2], 10, 64)
			return shop, item, true
		}
	}
	return 0, 0, false
}

func isShopeeHost(h string) bool { return h == "shopee.sg" || h == "www.shopee.sg" }

func canonical(shop, item int64) string {
	return fmt.Sprintf("https://shopee.sg/product/%d/%d", shop, item)
}

// Canonical keeps short s.shopee.sg links as they are. Preview reloads them and
// canonicalizes from where the browser ended up.
func (r *Reader) Canonical(raw string) (string, bool) {
	if shop, item, ok := ParseIDs(raw); ok {
		return canonical(shop, item), true
	}
	if u, err := url.Parse(raw); err == nil && u.Host == "s.shopee.sg" {
		return raw, true
	}
	return "", false
}

func (r *Reader) Read(ctx context.Context, b reader.Browser, rawURL string, want []string) (reader.Listing, error) {
	sleep := r.Sleep
	if sleep == nil {
		sleep = func(time.Duration) { time.Sleep(time.Duration(5+rand.IntN(16)) * time.Second) }
	}
	loads := want
	if len(loads) == 0 {
		loads = []string{""}
	}

	var merged reader.Listing
	for i, model := range loads {
		if i > 0 {
			sleep(0)
		}
		l, err := r.readOnce(ctx, b, rawURL, model)
		if err != nil {
			return reader.Listing{}, err
		}
		if i == 0 {
			merged = l
			continue
		}
		// Keep the voucher price for the model this load asked for.
		for _, v := range l.Variants {
			if v.ExternalID != model || v.BestIsEstimate {
				continue
			}
			for j := range merged.Variants {
				if merged.Variants[j].ExternalID == model {
					merged.Variants[j] = v
				}
			}
		}
	}
	return merged, reader.Validate(merged)
}

func (r *Reader) readOnce(ctx context.Context, b reader.Browser, rawURL, model string) (reader.Listing, error) {
	target := rawURL
	if model != "" {
		target += "?extraParams=" + url.QueryEscape(fmt.Sprintf(`{"display_model_id":%s,"model_selection_logic":3}`, model))
	}
	page, err := b.Load(ctx, target, []string{apiPath})
	if err != nil {
		return reader.Listing{}, err
	}
	if strings.Contains(page.FinalURL, "/verify/") || strings.Contains(page.FinalURL, "/buyer/login") {
		if strings.Contains(page.FinalURL, "is_logged_in=false") || strings.Contains(page.FinalURL, "/buyer/login") {
			return reader.Listing{}, reader.ErrLoggedOut
		}
		return reader.Listing{}, reader.ErrBlocked
	}
	var body []byte
	for u, b := range page.Responses {
		if strings.Contains(u, apiPath) {
			body = b
		}
	}
	if body == nil {
		return reader.Listing{}, fmt.Errorf("%w: no product data on page", reader.ErrUnreadable)
	}
	l, err := ParseGetPC(body)
	if err != nil {
		return l, err
	}
	l.URL = rawURL
	if shop, item, ok := ParseIDs(page.FinalURL); ok {
		l.URL = canonical(shop, item)
	}
	return l, nil
}

type getPC struct {
	Error any `json:"error"`
	Data  *struct {
		Item struct {
			Title        string `json:"title"`
			StockDisplay string `json:"stock_display"`
			Models       []struct {
				ModelID             int64  `json:"model_id"`
				Name                string `json:"name"`
				Price               int64  `json:"price"`
				PriceBeforeDiscount int64  `json:"price_before_discount"`
				Status              int    `json:"status"`
			} `json:"models"`
		} `json:"item"`
		ShopDetailed struct {
			Name string `json:"name"`
		} `json:"shop_detailed"`
		ProductPrice struct {
			HasFinalPrice bool `json:"has_final_price"`
			Price         struct {
				SingleValue int64 `json:"single_value"`
			} `json:"price"`
			FinalPriceInfo *struct {
				ModelID int64 `json:"model_id"`
			} `json:"final_price_info"`
		} `json:"product_price"`
	} `json:"data"`
}

// ParseGetPC turns a get_pc response into a listing.
func ParseGetPC(body []byte) (reader.Listing, error) {
	var doc getPC
	if err := json.Unmarshal(body, &doc); err != nil {
		return reader.Listing{}, fmt.Errorf("%w: %v", reader.ErrUnreadable, err)
	}
	if doc.Data == nil {
		return reader.Listing{}, fmt.Errorf("%w: empty product data (error %v)", reader.ErrUnreadable, doc.Error)
	}
	d := doc.Data
	var voucherModel int64 = -1
	if d.ProductPrice.HasFinalPrice && d.ProductPrice.FinalPriceInfo != nil {
		voucherModel = d.ProductPrice.FinalPriceInfo.ModelID
	}
	soldOut := strings.EqualFold(d.Item.StockDisplay, "SOLD OUT")

	l := reader.Listing{Site: reader.Shopee, Title: clean(d.Item.Title), Seller: clean(d.ShopDetailed.Name)}
	for _, m := range d.Item.Models {
		v := reader.VariantPrice{
			ExternalID: strconv.FormatInt(m.ModelID, 10),
			Name:       clean(m.Name),
			PriceCents: m.Price / 1000,
			InStock:    m.Status == 1 && !soldOut,
		}
		if m.PriceBeforeDiscount != m.Price {
			v.OriginalCents = m.PriceBeforeDiscount / 1000
		}
		if m.ModelID == voucherModel && d.ProductPrice.Price.SingleValue > 0 {
			v.BestCents = d.ProductPrice.Price.SingleValue / 1000
		} else {
			v.BestCents = v.PriceCents
			v.BestIsEstimate = true
		}
		l.Variants = append(l.Variants, v)
	}
	return l, reader.Validate(l)
}

// clean collapses any whitespace, including the non-breaking spaces Shopee
// puts in titles, to single spaces.
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }
