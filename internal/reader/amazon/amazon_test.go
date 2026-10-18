package amazon

import (
	"errors"
	"os"
	"testing"

	"dropcatch/internal/reader"
)

func TestCoupon(t *testing.T) {
	cases := []struct {
		text  string
		price int64
		want  int64
	}{
		{"Apply S$20.00 coupon", 27500, 2000},
		{"Apply 10% coupon", 27500, 2750},
		{"apply s$5 voucher Terms", 27500, 500},
		{"Save 5% on 2 items", 27500, 0},
		{"", 27500, 0},
	}
	for _, c := range cases {
		if got := Coupon(c.text, c.price); got != c.want {
			t.Errorf("Coupon(%q) = %d; want %d", c.text, got, c.want)
		}
	}
}

func TestCanonical(t *testing.T) {
	r := New()
	for _, raw := range []string{
		"https://www.amazon.sg/dp/B0FHWNZLKT",
		"https://www.amazon.sg/Nothing-Ear-Wireless/dp/B0FHWNZLKT/ref=sr_1_1?keywords=x",
		"https://amazon.sg/gp/product/B0FHWNZLKT?th=1",
	} {
		if got, ok := r.Canonical(raw); !ok || got != "https://www.amazon.sg/dp/B0FHWNZLKT" {
			t.Errorf("Canonical(%q) = %q %v", raw, got, ok)
		}
	}
	if _, ok := r.Canonical("https://www.amazon.com/dp/B0FHWNZLKT"); ok {
		t.Error("amazon.com is not Amazon SG")
	}
}

func TestParseHTML(t *testing.T) {
	b, err := os.ReadFile("testdata/nothing_ear3.html")
	if err != nil {
		t.Fatal(err)
	}
	l, err := ParseHTML(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if l.Seller != "Amazon US" || len(l.Variants) != 1 || l.Title[:15] != "Nothing Ear (3)" {
		t.Fatalf("listing = %+v", l)
	}
	want := reader.VariantPrice{ExternalID: "B0FHWNZLKT", Name: "", PriceCents: 17921, BestCents: 17921, InStock: true}
	if l.Variants[0] != want {
		t.Fatalf("variant = %+v", l.Variants[0])
	}
}

func TestParseHTMLWithCouponAndListPrice(t *testing.T) {
	html := `<input type="hidden" id="ASIN" name="ASIN" value="B000TEST01">
<span id="productTitle"> Thing </span>
<div id="corePrice_feature_div"><span class="a-offscreen">S$100.00</span></div>
<div id="corePriceDisplay_desktop_feature_div"><span class="basisPrice"><span class="a-offscreen">S$150.00</span></span></div>
<div id="promoPriceBlockMessage_feature_div"><label id="couponTextpctch">Apply S$10.00 coupon</label></div>
<div id="availability"><span>Currently unavailable.</span></div>`
	l, err := ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	v := l.Variants[0]
	if v.PriceCents != 10000 || v.BestCents != 9000 || v.OriginalCents != 15000 || v.InStock {
		t.Fatalf("variant = %+v", v)
	}
}

func TestParseHTMLNoPrice(t *testing.T) {
	_, err := ParseHTML(`<input id="ASIN" name="ASIN" value="B000TEST01"><span id="productTitle">X</span>`)
	if !errors.Is(err, reader.ErrUnreadable) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseHTMLBlocked(t *testing.T) {
	_, err := ParseHTML(`<h4>Enter the characters you see below</h4>`)
	if !errors.Is(err, reader.ErrBlocked) {
		t.Fatalf("err = %v", err)
	}
}
