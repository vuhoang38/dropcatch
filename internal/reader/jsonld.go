package reader

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"dropcatch/internal/money"
)

var jsonLDRe = regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// ProductFromJSONLD finds the first schema.org Product in the page's JSON-LD
// and returns its name, first offer price and stock.
func ProductFromJSONLD(html string) (name string, priceCents int64, inStock bool, ok bool) {
	for _, m := range jsonLDRe.FindAllStringSubmatch(html, -1) {
		var doc any
		if json.Unmarshal([]byte(strings.TrimSpace(m[1])), &doc) != nil {
			continue
		}
		if p := findProduct(doc); p != nil {
			offer := firstOffer(p["offers"])
			if offer == nil {
				continue
			}
			cents, err := money.ParseSGD(fmt.Sprint(offer["price"]))
			if err != nil {
				continue
			}
			n, _ := p["name"].(string)
			avail, _ := offer["availability"].(string)
			return n, cents, strings.HasSuffix(avail, "InStock"), true
		}
	}
	return "", 0, false, false
}

func findProduct(doc any) map[string]any {
	switch d := doc.(type) {
	case []any:
		for _, item := range d {
			if p := findProduct(item); p != nil {
				return p
			}
		}
	case map[string]any:
		if isType(d["@type"], "Product") {
			return d
		}
		// Look inside wrappers like @graph and ItemPage.mainEntity.
		for _, k := range []string{"@graph", "mainEntity", "itemOffered"} {
			if inner, ok := d[k]; ok {
				if p := findProduct(inner); p != nil {
					return p
				}
			}
		}
	}
	return nil
}

func isType(t any, want string) bool {
	switch v := t.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, _ := x.(string); s == want {
				return true
			}
		}
	}
	return false
}

func firstOffer(o any) map[string]any {
	switch v := o.(type) {
	case map[string]any:
		if inner, ok := v["offers"]; ok && v["price"] == nil { // AggregateOffer
			if got := firstOffer(inner); got != nil {
				return got
			}
			if low, ok := v["lowPrice"]; ok {
				return map[string]any{"price": low, "availability": v["availability"]}
			}
		}
		return v
	case []any:
		if len(v) > 0 {
			return firstOffer(v[0])
		}
	}
	return nil
}
