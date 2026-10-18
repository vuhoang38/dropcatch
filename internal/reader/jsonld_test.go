package reader

import "testing"

func TestProductFromJSONLD(t *testing.T) {
	cases := map[string]string{
		"object": `<html><script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"X","offers":{"@type":"Offer","price":"1299.00","availability":"https://schema.org/InStock"}}</script></html>`,
		"array":  `<script type="application/ld+json">[{"@type":"BreadcrumbList"},{"@type":"Product","name":"X","offers":[{"price":1299,"availability":"http://schema.org/InStock"}]}]</script>`,
		"graph":  `<script type='application/ld+json'>{"@graph":[{"@type":"WebPage"},{"@type":["Product"],"name":"X","offers":{"price":"1,299.00","availability":"InStock"}}]}</script>`,
	}
	for name, html := range cases {
		n, cents, inStock, ok := ProductFromJSONLD(html)
		if !ok || n != "X" || cents != 129900 || !inStock {
			t.Errorf("%s: %q %d %v %v", name, n, cents, inStock, ok)
		}
	}
	_, _, inStock, ok := ProductFromJSONLD(`<script type="application/ld+json">{"@type":"Product","name":"X","offers":{"price":"10","availability":"https://schema.org/OutOfStock"}}</script>`)
	if !ok || inStock {
		t.Errorf("out of stock: %v %v", inStock, ok)
	}
	if _, _, _, ok := ProductFromJSONLD(`<p>nothing</p>`); ok {
		t.Error("no json-ld should not be ok")
	}
}
