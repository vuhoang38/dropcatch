package shopee

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"dropcatch/internal/reader"
)

func fixture(t *testing.T, mutate func(data map[string]any)) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/get_pc_airpods.json")
	if err != nil {
		t.Fatal(err)
	}
	if mutate == nil {
		return b
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	mutate(doc["data"].(map[string]any))
	out, _ := json.Marshal(doc)
	return out
}

func TestParseIDs(t *testing.T) {
	for _, raw := range []string{
		"https://shopee.sg/Apple-AirPods-Pro-3-i.442800909.40818293338?extraParams=x",
		"https://shopee.sg/product/442800909/40818293338",
		"shopee.sg/product/442800909/40818293338?utm_source=a",
	} {
		shop, item, ok := ParseIDs(raw)
		if !ok || shop != 442800909 || item != 40818293338 {
			t.Errorf("ParseIDs(%q) = %d %d %v", raw, shop, item, ok)
		}
	}
	if _, _, ok := ParseIDs("https://shopee.sg/"); ok {
		t.Error("home page should not parse")
	}
	if _, _, ok := ParseIDs("https://lazada.sg/product/1/2"); ok {
		t.Error("other site should not parse")
	}
}

func TestCanonical(t *testing.T) {
	r := New()
	got, ok := r.Canonical("https://shopee.sg/Apple-AirPods-Pro-3-i.442800909.40818293338?sp_atk=1")
	if !ok || got != "https://shopee.sg/product/442800909/40818293338" {
		t.Fatalf("canonical = %q %v", got, ok)
	}
	if got, ok := r.Canonical("https://s.shopee.sg/abc123"); !ok || got != "https://s.shopee.sg/abc123" {
		t.Fatalf("short link = %q %v", got, ok)
	}
}

func TestParseGetPCAirpods(t *testing.T) {
	l, err := ParseGetPC(fixture(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Apple AirPods Pro 3" || l.Seller != "Apple Flagship Store" || len(l.Variants) != 1 {
		t.Fatalf("listing = %+v", l)
	}
	want := reader.VariantPrice{ExternalID: "261503633746", Name: "Pro 3", PriceCents: 31600, BestCents: 31500, OriginalCents: 34900, InStock: true}
	if l.Variants[0] != want {
		t.Fatalf("variant = %+v\nwant      %+v", l.Variants[0], want)
	}
	if err := reader.Validate(l); err != nil {
		t.Fatal(err)
	}
}

func TestParseGetPCVoucherForOtherModel(t *testing.T) {
	l, err := ParseGetPC(fixture(t, func(d map[string]any) {
		d["product_price"].(map[string]any)["final_price_info"].(map[string]any)["model_id"] = 999
	}))
	if err != nil {
		t.Fatal(err)
	}
	v := l.Variants[0]
	if v.BestCents != 31600 || !v.BestIsEstimate {
		t.Fatalf("variant = %+v", v)
	}
}

func TestParseGetPCZeroPrice(t *testing.T) {
	l, err := ParseGetPC(fixture(t, func(d map[string]any) {
		d["item"].(map[string]any)["models"].([]any)[0].(map[string]any)["price"] = 0
	}))
	if err == nil {
		err = reader.Validate(l)
	}
	if !errors.Is(err, reader.ErrUnreadable) {
		t.Fatalf("err = %v, want ErrUnreadable", err)
	}
}

func TestParseGetPCSoldOut(t *testing.T) {
	l, _ := ParseGetPC(fixture(t, func(d map[string]any) {
		d["item"].(map[string]any)["stock_display"] = "SOLD OUT"
	}))
	if l.Variants[0].InStock {
		t.Fatal("want out of stock")
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
		p.Responses["https://shopee.sg/api/v4/pdp/get_pc?item_id=1"] = f.body
	}
	return p, nil
}

func noSleep(time.Duration) {}

func TestReadDetectsLogin(t *testing.T) {
	r := &Reader{Sleep: noSleep}
	b := &fakeBrowser{final: "https://shopee.sg/verify/traffic/error?home_url=x&is_logged_in=false&type=4"}
	if _, err := r.Read(context.Background(), b, "https://shopee.sg/product/1/2", nil); !errors.Is(err, reader.ErrLoggedOut) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadDetectsBlock(t *testing.T) {
	r := &Reader{Sleep: noSleep}
	b := &fakeBrowser{final: "https://shopee.sg/verify/captcha?x=1"}
	if _, err := r.Read(context.Background(), b, "https://shopee.sg/product/1/2", nil); !errors.Is(err, reader.ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadMissingResponse(t *testing.T) {
	r := &Reader{Sleep: noSleep}
	if _, err := r.Read(context.Background(), &fakeBrowser{}, "https://shopee.sg/product/1/2", nil); !errors.Is(err, reader.ErrUnreadable) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadWantedModelUsesDisplayParam(t *testing.T) {
	r := &Reader{Sleep: noSleep}
	b := &fakeBrowser{body: fixture(t, nil)}
	l, err := r.Read(context.Background(), b, "https://shopee.sg/product/442800909/40818293338", []string{"261503633746"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.urls) != 1 || !strings.Contains(b.urls[0], "display_model_id%22%3A261503633746%2C%22model_selection_logic%22%3A3") {
		t.Fatalf("urls = %v", b.urls)
	}
	if l.Variants[0].BestCents != 31500 || l.URL != "https://shopee.sg/product/442800909/40818293338" {
		t.Fatalf("listing = %+v", l)
	}
}

func TestParseGetPCVariantSoldOut(t *testing.T) {
	for _, field := range []string{"has_stock", "is_grayout"} {
		l, _ := ParseGetPC(fixture(t, func(d map[string]any) {
			m := d["item"].(map[string]any)["models"].([]any)[0].(map[string]any)
			if field == "has_stock" {
				m["has_stock"] = false
			} else {
				m["is_grayout"] = true
			}
		}))
		if l.Variants[0].InStock {
			t.Errorf("%s: variant should be out of stock", field)
		}
	}
}
