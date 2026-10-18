package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"dropcatch/internal/alert"
	"dropcatch/internal/checker"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/store"
)

type SparkPoint struct {
	Day   string `json:"day"`
	Cents int64  `json:"cents"`
}

type ProductSummary struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	Paused       bool         `json:"paused"`
	LinkCount    int          `json:"linkCount"`
	BestCents    *int64       `json:"bestCents"`
	BestSite     string       `json:"bestSite"`
	TypicalCents *int64       `json:"typicalCents"`
	PctVsTypical *float64     `json:"pctVsTypical"` // negative = cheaper than typical
	Spark        []SparkPoint `json:"spark"`
	Warnings     []string     `json:"warnings"`
}

type AlertSettings struct {
	DropPct        float64 `json:"dropPct"`
	WindowDays     int     `json:"windowDays"`
	MinHistoryDays int     `json:"minHistoryDays"`
}

type Overrides struct {
	DropPct        *float64 `json:"dropPct"`
	WindowDays     *int     `json:"windowDays"`
	MinHistoryDays *int     `json:"minHistoryDays"`
}

type VariantDetail struct {
	ID             int64      `json:"id"`
	ExternalID     string     `json:"externalId"`
	Name           string     `json:"name"`
	Tracked        bool       `json:"tracked"`
	IsNew          bool       `json:"isNew"`
	Gone           bool       `json:"gone"`
	InStock        *bool      `json:"inStock"`
	PriceCents     *int64     `json:"priceCents"`
	BestCents      *int64     `json:"bestCents"`
	BestIsEstimate bool       `json:"bestIsEstimate"`
	OriginalCents  *int64     `json:"originalCents"`
	CheckedAt      *time.Time `json:"checkedAt"`
}

type LinkDetail struct {
	ID            int64           `json:"id"`
	URL           string          `json:"url"`
	Site          string          `json:"site"`
	Title         string          `json:"title"`
	Seller        string          `json:"seller"`
	Status        string          `json:"status"`
	LastError     string          `json:"lastError"`
	LastCheckedAt *time.Time      `json:"lastCheckedAt"`
	Variants      []VariantDetail `json:"variants"`
}

type ProductDetail struct {
	ProductSummary
	Settings  AlertSettings `json:"settings"`
	Overrides Overrides     `json:"overrides"`
	Links     []LinkDetail  `json:"links"`
}

type HistoryPoint struct {
	T     time.Time `json:"t"`
	Cents int64     `json:"cents"`
}

type Series struct {
	VariantID int64          `json:"variantId"`
	Label     string         `json:"label"`
	Points    []HistoryPoint `json:"points"`
}

type History struct {
	Series       []Series `json:"series"`
	TypicalCents *int64   `json:"typicalCents"`
}

type PreviewVariant struct {
	ExternalID     string `json:"externalId"`
	Name           string `json:"name"`
	PriceCents     int64  `json:"priceCents"`
	BestCents      int64  `json:"bestCents"`
	BestIsEstimate bool   `json:"bestIsEstimate"`
	InStock        bool   `json:"inStock"`
}

type Preview struct {
	Site     string           `json:"site"`
	URL      string           `json:"url"`
	Title    string           `json:"title"`
	Seller   string           `json:"seller"`
	Variants []PreviewVariant `json:"variants"`
}

// productState is everything the summary and detail views need.
type productState struct {
	p        store.Product
	links    []store.Link
	variants map[int64][]store.Variant // by link id
	latest   map[int64]store.Reading   // by variant id
}

func (s *server) load(ctx context.Context, p store.Product) (productState, error) {
	st := productState{p: p, variants: map[int64][]store.Variant{}}
	var err error
	if st.links, err = s.Store.ListLinks(ctx, p.ID); err != nil {
		return st, err
	}
	for _, l := range st.links {
		if st.variants[l.ID], err = s.Store.ListVariants(ctx, l.ID); err != nil {
			return st, err
		}
	}
	st.latest, err = s.Store.LatestReadings(ctx, p.ID)
	return st, err
}

func (s *server) summary(ctx context.Context, st productState) (ProductSummary, error) {
	out := ProductSummary{ID: st.p.ID, Name: st.p.Name, Paused: st.p.Paused, LinkCount: len(st.links), Spark: []SparkPoint{}, Warnings: []string{}}

	broken, newVariants := 0, 0
	sites := map[string]bool{}
	for _, l := range st.links {
		sites[l.Site] = true
		if l.Status == store.LinkBroken {
			broken++
		}
		for _, v := range st.variants[l.ID] {
			if v.IsNew {
				newVariants++
			}
			r, ok := st.latest[v.ID]
			if !v.Tracked || v.Gone || !ok || !r.InStock {
				continue
			}
			if out.BestCents == nil || r.BestCents < *out.BestCents {
				best := r.BestCents
				out.BestCents, out.BestSite = &best, l.Site
			}
		}
	}

	now := s.Now()
	in, err := checker.AlertInput(ctx, s.Store, st.p, now, out.BestCents)
	if err != nil {
		return out, err
	}
	if res := alert.Evaluate(in); res.HasTypical {
		typical := res.TypicalCents
		out.TypicalCents = &typical
		if out.BestCents != nil {
			pct := float64(*out.BestCents-typical) / float64(typical) * 100
			out.PctVsTypical = &pct
		}
	}

	today := alert.DayStart(now)
	daily, err := s.Store.DailyCheapest(ctx, st.p.ID, today.AddDate(0, 0, -29), today.AddDate(0, 0, 1))
	if err != nil {
		return out, err
	}
	for _, d := range daily {
		out.Spark = append(out.Spark, SparkPoint{Day: d.Day, Cents: d.Cents})
	}

	if broken > 0 {
		out.Warnings = append(out.Warnings, plural(broken, "link")+" broken")
	}
	for _, site := range sortedKeys(sites) {
		switch status, _ := s.Store.GetSession(ctx, site); status {
		case store.SessionExpired:
			out.Warnings = append(out.Warnings, notify.SiteName(site)+" login expired")
		case store.SessionBlocked:
			out.Warnings = append(out.Warnings, notify.SiteName(site)+" needs verification")
		}
	}
	if newVariants > 0 {
		out.Warnings = append(out.Warnings, plural(newVariants, "new variant"))
	}
	return out, nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func (s *server) detail(ctx context.Context, id int64) (ProductDetail, error) {
	p, err := s.Store.GetProduct(ctx, id)
	if err != nil {
		return ProductDetail{}, err
	}
	st, err := s.load(ctx, p)
	if err != nil {
		return ProductDetail{}, err
	}
	sum, err := s.summary(ctx, st)
	if err != nil {
		return ProductDetail{}, err
	}
	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		return ProductDetail{}, err
	}
	params := checker.Params(p, settings)
	out := ProductDetail{
		ProductSummary: sum,
		Settings:       AlertSettings{DropPct: params.DropPct, WindowDays: params.WindowDays, MinHistoryDays: params.MinHistoryDays},
		Overrides:      Overrides{DropPct: p.DropPct, WindowDays: p.WindowDays, MinHistoryDays: p.MinHistoryDays},
		Links:          []LinkDetail{},
	}
	for _, l := range st.links {
		ld := LinkDetail{ID: l.ID, URL: l.URL, Site: l.Site, Title: l.Title, Seller: l.Seller,
			Status: string(l.Status), LastError: l.LastError, LastCheckedAt: l.LastCheckedAt, Variants: []VariantDetail{}}
		for _, v := range st.variants[l.ID] {
			vd := VariantDetail{ID: v.ID, ExternalID: v.ExternalID, Name: v.Name, Tracked: v.Tracked, IsNew: v.IsNew, Gone: v.Gone}
			if r, ok := st.latest[v.ID]; ok {
				inStock, price, best, at := r.InStock, r.PriceCents, r.BestCents, r.At
				vd.InStock, vd.PriceCents, vd.BestCents, vd.CheckedAt = &inStock, &price, &best, &at
				vd.BestIsEstimate, vd.OriginalCents = r.BestIsEstimate, r.OriginalCents
			}
			ld.Variants = append(ld.Variants, vd)
		}
		out.Links = append(out.Links, ld)
	}
	return out, nil
}

func (s *server) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.Store.ListProducts(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := []ProductSummary{}
	for _, p := range products {
		st, err := s.load(r.Context(), p)
		if err != nil {
			fail(w, err)
			return
		}
		sum, err := s.summary(r.Context(), st)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, sum)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		var d ProductDetail
		if d, err = s.detail(r.Context(), id); err == nil {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	fail(w, err)
}

type linkInput struct {
	URL        string   `json:"url"`
	VariantIDs []string `json:"variantIds"`
}

// previewCached reuses a preview from the last 30 minutes, so saving a
// product doesn't load every page a second time.
func (s *server) previewCached(ctx context.Context, raw string) (reader.Listing, error) {
	if _, canon, err := reader.For(raw); err == nil {
		s.previewMu.Lock()
		c, ok := s.previews[canon]
		s.previewMu.Unlock()
		if ok && s.Now().Sub(c.at) < 30*time.Minute {
			return c.listing, nil
		}
	}
	l, err := s.Checker.Preview(ctx, raw)
	if err != nil {
		return l, err
	}
	s.previewMu.Lock()
	s.previews[l.URL] = cachedPreview{listing: l, at: s.Now()}
	s.previewMu.Unlock()
	return l, nil
}

func (s *server) saveLink(ctx context.Context, productID int64, l reader.Listing, tracked []string) error {
	link, err := s.Store.CreateLink(ctx, productID, l.URL, string(l.Site), l.Title, l.Seller)
	if err != nil {
		return err
	}
	for _, v := range l.Variants {
		_, err := s.Store.UpsertVariant(ctx, store.Variant{
			LinkID: link.ID, ExternalID: v.ExternalID, Name: v.Name, Tracked: slices.Contains(tracked, v.ExternalID),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) createProduct(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string      `json:"name"`
		Links []linkInput `json:"links"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "A product needs a name")
		return
	}
	ctx := r.Context()
	// Read every link first, so a bad link doesn't leave half a product.
	listings := make([]reader.Listing, len(in.Links))
	seen := map[string]bool{}
	for i, li := range in.Links {
		l, err := s.previewCached(ctx, li.URL)
		if err != nil {
			fail(w, fmt.Errorf("%s: %w", li.URL, err))
			return
		}
		if seen[l.URL] {
			fail(w, store.ErrDuplicate)
			return
		}
		seen[l.URL] = true
		listings[i] = l
	}
	p, err := s.Store.CreateProduct(ctx, strings.TrimSpace(in.Name))
	if err != nil {
		fail(w, err)
		return
	}
	for i, l := range listings {
		if err := s.saveLink(ctx, p.ID, l, in.Links[i].VariantIDs); err != nil {
			_ = s.Store.DeleteProduct(ctx, p.ID)
			fail(w, err)
			return
		}
	}
	s.respondDetail(w, http.StatusCreated, p.ID)
}

func (s *server) respondDetail(w http.ResponseWriter, code int, id int64) {
	d, err := s.detail(context.Background(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, code, d)
}

func (s *server) addLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		_, err = s.Store.GetProduct(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	var in linkInput
	if err := readJSON(r, &in); err != nil || in.URL == "" {
		writeError(w, http.StatusBadRequest, "Paste a link")
		return
	}
	l, err := s.previewCached(r.Context(), in.URL)
	if err == nil {
		err = s.saveLink(r.Context(), id, l, in.VariantIDs)
	}
	if err != nil {
		fail(w, err)
		return
	}
	s.respondDetail(w, http.StatusCreated, id)
}

func (s *server) patchProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	var p store.Product
	if err == nil {
		p, err = s.Store.GetProduct(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	var in map[string]json.RawMessage
	if err := readJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "Bad request body")
		return
	}
	bad := func(field string) { writeError(w, http.StatusBadRequest, "Invalid "+field) }
	if raw, ok := in["name"]; ok {
		if json.Unmarshal(raw, &p.Name) != nil || strings.TrimSpace(p.Name) == "" {
			bad("name")
			return
		}
	}
	if raw, ok := in["paused"]; ok && json.Unmarshal(raw, &p.Paused) != nil {
		bad("paused")
		return
	}
	// An explicit null clears an override.
	if raw, ok := in["dropPct"]; ok {
		if json.Unmarshal(raw, &p.DropPct) != nil || (p.DropPct != nil && (*p.DropPct <= 0 || *p.DropPct >= 100)) {
			bad("drop percent")
			return
		}
	}
	if raw, ok := in["windowDays"]; ok {
		if json.Unmarshal(raw, &p.WindowDays) != nil || (p.WindowDays != nil && *p.WindowDays < 1) {
			bad("window days")
			return
		}
	}
	if raw, ok := in["minHistoryDays"]; ok {
		if json.Unmarshal(raw, &p.MinHistoryDays) != nil || (p.MinHistoryDays != nil && *p.MinHistoryDays < 0) {
			bad("minimum history days")
			return
		}
	}
	if err := s.Store.UpdateProduct(r.Context(), p); err != nil {
		fail(w, err)
		return
	}
	s.respondDetail(w, http.StatusOK, id)
}

func (s *server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = s.Store.DeleteProduct(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deleteLink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = s.Store.DeleteLink(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) patchVariant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, err)
		return
	}
	var in struct {
		Tracked *bool `json:"tracked"`
	}
	if err := readJSON(r, &in); err != nil || in.Tracked == nil {
		writeError(w, http.StatusBadRequest, "Send tracked: true or false")
		return
	}
	if err := s.Store.SetVariantTracked(r.Context(), id, *in.Tracked); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) history(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	var p store.Product
	if err == nil {
		p, err = s.Store.GetProduct(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	now := s.Now()
	since := time.Time{}
	switch r.URL.Query().Get("range") {
	case "7d":
		since = now.AddDate(0, 0, -7)
	case "90d":
		since = now.AddDate(0, 0, -90)
	case "all":
	default:
		since = now.AddDate(0, 0, -30)
	}
	ctx := r.Context()
	st, err := s.load(ctx, p)
	if err != nil {
		fail(w, err)
		return
	}
	readings, err := s.Store.History(ctx, id, since)
	if err != nil {
		fail(w, err)
		return
	}
	out := History{Series: []Series{}}
	for _, l := range st.links {
		for _, v := range st.variants[l.ID] {
			if !v.Tracked || v.Gone {
				continue
			}
			var parts []string
			for _, s := range []string{notify.SiteName(l.Site), l.Seller, v.Name} {
				if s != "" {
					parts = append(parts, s)
				}
			}
			ser := Series{VariantID: v.ID, Label: strings.Join(parts, " · "), Points: []HistoryPoint{}}
			for _, rd := range readings[v.ID] {
				ser.Points = append(ser.Points, HistoryPoint{T: rd.At, Cents: rd.BestCents})
			}
			out.Series = append(out.Series, ser)
		}
	}
	sum, err := s.summary(ctx, st)
	if err != nil {
		fail(w, err)
		return
	}
	out.TypicalCents = sum.TypicalCents
	writeJSON(w, http.StatusOK, out)
}

func (s *server) checkNow(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		_, err = s.Store.GetProduct(r.Context(), id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	go func() {
		if err := s.Checker.CheckProduct(context.Background(), id); err != nil {
			log.Printf("check now %d: %v", id, err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (s *server) preview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.URL) == "" {
		writeError(w, http.StatusBadRequest, "Paste a link")
		return
	}
	l, err := s.previewCached(r.Context(), strings.TrimSpace(in.URL))
	if err != nil {
		fail(w, err)
		return
	}
	out := Preview{Site: string(l.Site), URL: l.URL, Title: l.Title, Seller: l.Seller, Variants: []PreviewVariant{}}
	for _, v := range l.Variants {
		out.Variants = append(out.Variants, PreviewVariant{ExternalID: v.ExternalID, Name: v.Name, PriceCents: v.PriceCents,
			BestCents: v.BestCents, BestIsEstimate: v.BestIsEstimate, InStock: v.InStock})
	}
	writeJSON(w, http.StatusOK, out)
}
