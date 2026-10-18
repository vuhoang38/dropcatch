// Package lazada reads prices from lazada.sg.
//
// The product page calls mtop.global.detail.web.getdetailinfo. Its data.module
// field is a JSON string holding, among others:
//   - product.title, seller.name
//   - productOption.skuBase.skus[]: skuId and propPath ("pid:vid;pid:vid")
//   - productOption.skuBase.properties[]: pid, values[] of vid and name
//   - skuInfos[skuId].price.salePrice/originalPrice with "$322.00" text
//   - skuInfos[skuId].operation.text: "Add to Cart" when buyable, "Add to
//     Wishlist" when it isn't
//   - primaryKey.loadedSkuIds: the skus skuInfos covers (sometimes all of them,
//     sometimes only the one on screen)
//   - user.login: false when the session is gone
//
// Checked by hand on 2026-10-18, logged in, on AirPods Pro 3: Lazada lists
// vouchers on the page but shows no "after voucher" price, so the best price
// is the sale price and isn't marked as an estimate.
package lazada

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dropcatch/internal/money"
	"dropcatch/internal/reader"
)

const apiPath = "mtop.global.detail.web.getdetailinfo"

type Reader struct {
	// Sleep pauses between page loads. Nil means a random 5 to 20 seconds.
	Sleep func(time.Duration)
}

func New() *Reader { return &Reader{} }

func (r *Reader) Site() reader.Site { return reader.Lazada }

var idsRe = regexp.MustCompile(`-i(\d+)(?:-s(\d+))?\.html$`)

// ParseIDs pulls the item id, and the sku id when present, out of a product URL.
func ParseIDs(raw string) (itemID, skuID string, ok bool) {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Host != "www.lazada.sg" && u.Host != "lazada.sg") || !strings.HasPrefix(u.Path, "/products/") {
		return "", "", false
	}
	m := idsRe.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func pageURL(item, sku string) string {
	if sku != "" {
		return fmt.Sprintf("https://www.lazada.sg/products/pdp-i%s-s%s.html", item, sku)
	}
	return fmt.Sprintf("https://www.lazada.sg/products/pdp-i%s.html", item)
}

func (r *Reader) Canonical(raw string) (string, bool) {
	item, _, ok := ParseIDs(raw)
	if !ok {
		return "", false
	}
	return pageURL(item, ""), true
}

func (r *Reader) Read(ctx context.Context, b reader.Browser, rawURL string, want []string) (reader.Listing, error) {
	item, _, ok := ParseIDs(rawURL)
	if !ok {
		return reader.Listing{}, fmt.Errorf("%w: not a Lazada product link", reader.ErrUnreadable)
	}
	l, err := r.readOnce(ctx, b, pageURL(item, ""))
	if err != nil {
		return l, err
	}
	l.URL = pageURL(item, "")

	// Some listings only price the sku on screen. Load the others you track.
	have := map[string]bool{}
	for _, v := range l.Variants {
		have[v.ExternalID] = true
	}
	for _, sku := range want {
		if have[sku] {
			continue
		}
		r.sleep()
		more, err := r.readOnce(ctx, b, pageURL(item, sku))
		if err != nil {
			return reader.Listing{}, err
		}
		for _, v := range more.Variants {
			if v.ExternalID == sku {
				l.Variants = append(l.Variants, v)
			}
		}
	}
	return l, reader.Validate(l)
}

func (r *Reader) sleep() {
	if r.Sleep != nil {
		r.Sleep(0)
		return
	}
	time.Sleep(time.Duration(5+rand.IntN(16)) * time.Second)
}

func (r *Reader) readOnce(ctx context.Context, b reader.Browser, target string) (reader.Listing, error) {
	page, err := b.Load(ctx, target, []string{apiPath})
	if err != nil {
		return reader.Listing{}, err
	}
	if u := strings.ToLower(page.FinalURL); strings.Contains(u, "punish") || strings.Contains(u, "captcha") || strings.Contains(u, "x5secdata") {
		return reader.Listing{}, reader.ErrBlocked
	}
	if strings.Contains(page.FinalURL, "member.lazada.sg/user/login") {
		return reader.Listing{}, reader.ErrLoggedOut
	}
	for u, body := range page.Responses {
		if strings.Contains(u, apiPath) {
			return ParseDetail(body)
		}
	}
	return reader.Listing{}, fmt.Errorf("%w: no product data on page", reader.ErrUnreadable)
}

type detail struct {
	Ret  []string `json:"ret"`
	Data struct {
		Module string `json:"module"`
	} `json:"data"`
}

type module struct {
	User struct {
		Login bool `json:"login"`
	} `json:"user"`
	Product struct {
		Title string `json:"title"`
	} `json:"product"`
	Seller struct {
		Name string `json:"name"`
	} `json:"seller"`
	ProductOption struct {
		SkuBase struct {
			Properties []struct {
				PID    string `json:"pid"`
				Values []struct {
					VID  string `json:"vid"`
					Name string `json:"name"`
				} `json:"values"`
			} `json:"properties"`
			Skus []struct {
				SkuID    string `json:"skuId"`
				PropPath string `json:"propPath"`
			} `json:"skus"`
		} `json:"skuBase"`
	} `json:"productOption"`
	SkuInfos map[string]struct {
		Price struct {
			SalePrice     struct{ Text string } `json:"salePrice"`
			OriginalPrice struct{ Text string } `json:"originalPrice"`
		} `json:"price"`
		Operation struct {
			Text    string `json:"text"`
			Disable bool   `json:"disable"`
		} `json:"operation"`
	} `json:"skuInfos"`
}

// ParseDetail turns a getdetailinfo response into a listing. Skus Lazada
// didn't price in this response are left out.
func ParseDetail(body []byte) (reader.Listing, error) {
	var d detail
	if err := json.Unmarshal(body, &d); err != nil || d.Data.Module == "" {
		return reader.Listing{}, fmt.Errorf("%w: empty product data %v", reader.ErrUnreadable, d.Ret)
	}
	var m module
	if err := json.Unmarshal([]byte(d.Data.Module), &m); err != nil {
		return reader.Listing{}, fmt.Errorf("%w: %v", reader.ErrUnreadable, err)
	}
	if !m.User.Login {
		return reader.Listing{}, reader.ErrLoggedOut
	}

	names := map[string]string{} // "pid:vid" -> value name
	for _, p := range m.ProductOption.SkuBase.Properties {
		for _, v := range p.Values {
			names[p.PID+":"+v.VID] = clean(v.Name)
		}
	}

	l := reader.Listing{Site: reader.Lazada, Title: clean(m.Product.Title), Seller: clean(m.Seller.Name)}
	for _, sku := range m.ProductOption.SkuBase.Skus {
		info, ok := m.SkuInfos[sku.SkuID]
		if !ok {
			continue
		}
		price, err := money.ParseSGD(info.Price.SalePrice.Text)
		if err != nil {
			return reader.Listing{}, fmt.Errorf("%w: sku %s: %v", reader.ErrUnreadable, sku.SkuID, err)
		}
		v := reader.VariantPrice{ExternalID: sku.SkuID, Name: variantName(sku.PropPath, names), PriceCents: price, BestCents: price}
		if orig, err := money.ParseSGD(info.Price.OriginalPrice.Text); err == nil && orig > price {
			v.OriginalCents = orig
		}
		op := strings.ToLower(info.Operation.Text)
		v.InStock = !info.Operation.Disable && !strings.Contains(op, "wishlist") && !strings.Contains(op, "out of stock")
		l.Variants = append(l.Variants, v)
	}
	return l, reader.Validate(l)
}

func variantName(propPath string, names map[string]string) string {
	var parts []string
	for _, kv := range strings.Split(propPath, ";") {
		if n := names[kv]; n != "" && n != "_" {
			parts = append(parts, n)
		}
	}
	return strings.Join(parts, " / ")
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }
