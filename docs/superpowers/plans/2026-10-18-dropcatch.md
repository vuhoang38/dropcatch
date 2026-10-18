# Dropcatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A personal price tracker that reads real prices from Shopee, Lazada, Amazon SG and Courts on a schedule, charts them, and emails you when a product drops well below its typical price.

**Architecture:** One Go binary. It drives your installed Chrome with chromedp through a dedicated profile, stores readings in SQLite, runs a scheduler and alert logic, sends email through Resend, and serves a JSON API plus the built React dashboard from `web/dist` (embedded). The dashboard is the vibecode template with its examples removed.

**Tech Stack:** Go 1.26, `github.com/chromedp/chromedp` v0.14.2 (pinned), `modernc.org/sqlite` (pure Go, no cgo), `github.com/PuerkitoBio/goquery` for HTML readers, `net/http` ServeMux with method patterns. Frontend from https://github.com/vuhoang38/vibecode-template (Vite, React 19, TypeScript, React Router, TanStack Query, Tailwind v4, shadcn/ui, Recharts, Bun).

**Spec:** `docs/superpowers/specs/2026-10-18-dropcatch-design.md`. Read it before starting any task.

## Global Constraints

- All money is `int64` cents of SGD. Shopee API values are units of 1/100000 SGD, so cents = value / 1000.
- Days are Singapore calendar days (UTC+8, no DST). Never bucket by UTC.
- Never use the seller's "original"/"was" price for alerts. Store it for display only.
- A failed read records a failure. It never records a price, and never a zero price.
- Out of stock, unticked, and gone variants never count toward the current best or the typical price.
- Alert defaults: drop 20%, window 30 days, minimum history 2 days. Check interval default 3 hours. Stale check limit 12 hours.
- Email via Resend, `POST https://api.resend.com/emails`, from `Dropcatch <onboarding@resend.dev>`.
- Chrome: your installed `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, not headless, flag `disable-blink-features=AutomationControlled`, offscreen with `window-position=-2400,-2400` during checks. One Chrome process at a time because one profile can't be opened twice.
- No scrolling, mouse movement or other human imitation. No paid captcha solving. Never retry through a captcha.
- Server listens on `127.0.0.1:8080` only.
- Data dir default `~/Library/Application Support/Dropcatch` holding `dropcatch.db`, `chrome-profile/`, `logs/`.
- Frontend follows the template's `web/CLAUDE.md` hard rules and definition of done (`bun run verify`, `bun run screenshot <route>` and look at the PNG). Bun lives at `~/.bun/bin`, so prefix commands with `export PATH="$HOME/.bun/bin:$PATH"`.
- Every commit message ends with:
  ```
  Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01U9FVS2x8H9eZgAZGjMZFZq
  ```

## Review Focus

1. A page loads but the price element is missing or parses to 0. Expect a failed read and a broken link, never a $0 reading (that would look like a 100% drop and email you). Pinned in Task 4 and Task 5.
2. Shopee returns the "After Voucher" price for a different model than the one requested (`final_price_info.model_id` mismatch). Expect that variant's best price to fall back to its plain model price with `BestIsEstimate = true`, never to borrow another variant's voucher price. Pinned in Task 5.
3. A check at 07:30 SGT, which is 23:30 UTC the day before. Expect it to land on the Singapore day. Pinned in Task 2.
4. Every link of a product fails or every variant is out of stock. Expect no alert, and the no spam state left unchanged, not reset. Pinned in Task 3 and Task 9.
5. Links pasted in other shapes, like `shopee.sg/product/<shop>/<item>`, a URL with tracking params, or a `s.shopee.sg` short link. Expect them normalized to one canonical URL so the same listing isn't tracked twice. Pinned in Task 5 and Task 13.

---

## File Structure

```
go.mod                               module dropcatch
cmd/dropcatch/main.go                flags, wiring, `serve` (default) and `install-agent`
cmd/capture/main.go                  dev tool: load a URL in the profile, save HTML and API responses to testdata
internal/money/money.go              ParseSGD, Format
internal/store/store.go              Open, migrations, Settings, KV
internal/store/schema.sql            embedded schema
internal/store/catalog.go            products, links, variants
internal/store/readings.go           readings, check log, daily cheapest, latest per variant
internal/store/notices.go            one-shot failure notice state
internal/alert/alert.go              pure alert rule
internal/reader/reader.go            Listing, VariantPrice, errors, Reader, Browser, registry
internal/reader/jsonld.go            shared JSON-LD Product extraction
internal/reader/shopee/shopee.go     URL normalize, get_pc parsing, Reader
internal/reader/lazada/lazada.go
internal/reader/amazon/amazon.go
internal/reader/courts/courts.go
internal/browser/chrome.go           chromedp Chrome implementing reader.Browser, OpenVisible
internal/notify/resend.go            Mailer, Resend client
internal/notify/emails.go            email bodies
internal/notify/notices.go           Raise/Clear one-shot emails
internal/checker/checker.go          CheckProduct, RunAll, Preview
internal/scheduler/scheduler.go      Due, Stale, Run loop
internal/server/server.go            routes, SPA fallback
internal/server/products.go          product, link, variant handlers
internal/server/settings.go          settings, test email, sessions, login
web/                                 the template, examples removed
web/embed.go                         package web, //go:embed all:dist
```

---

### Task 1: Repo, Go module, money

**Files:**
- Create: `.gitignore`, `go.mod`, `internal/money/money.go`, `internal/money/money_test.go`

**Interfaces:**
- Produces: `money.ParseSGD(s string) (int64, error)` accepts `"$315.00"`, `"S$1,299"`, `"SGD 27.50"`, `"$ 2,098.00"` and returns cents. Rejects empty strings and `0`. `money.Format(cents int64) string` returns `"$1,299.00"`.

- [ ] **Step 1:** `git init`, write `.gitignore` with `.superpowers/`, `web/node_modules/`, `web/dist/*` but keep `web/dist/.gitkeep`, `web/.screenshots/`, `/dropcatch`, `*.db`. `go mod init dropcatch`.
- [ ] **Step 2: Write the failing test** `TestParseSGD` table: `"$315.00"→31500`, `"S$1,299"→129900`, `"SGD 27.50"→2750`, `"$\n207.00"→20700`, `"$0.00"→error`, `""→error`, `"Free"→error`. `TestFormat`: `129900→"$1,299.00"`, `2750→"$27.50"`.
- [ ] **Step 3:** Run `go test ./internal/money/`. Expected FAIL, undefined.
- [ ] **Step 4:** Implement in `internal/money/money.go`. Strip `S`, `SGD`, `$`, commas, whitespace. Parse with integer math, not float.
- [ ] **Step 5:** Run `go test ./internal/money/`. Expected PASS.
- [ ] **Step 6:** Commit `chore: init repo and money helpers`.

### Task 2: Store

**Files:**
- Create: `internal/store/store.go`, `schema.sql`, `catalog.go`, `readings.go`, `notices.go`, and `store_test.go`

**Interfaces:**
- Consumes: `alert.DayPrice` and `alert.SGT`. Create `internal/alert/alert.go` in this task holding only those two declarations (exact shapes in Task 3). `store` imports `alert`, never the reverse.
- Produces:
```go
func Open(path string) (*Store, error)     // ":memory:" works for tests; WAL; foreign keys on
type Settings struct {
    DropPct float64; WindowDays, MinHistoryDays, CheckIntervalHours int
    ResendAPIKey, Email string
}
func DefaultSettings() Settings              // 20, 30, 2, 3, "", ""
func (s *Store) GetSettings(ctx) (Settings, error)
func (s *Store) PutSettings(ctx, Settings) error
func (s *Store) GetTime(ctx, key string) (time.Time, error)   // zero if unset; keys "last_run_at", "last_success_at"
func (s *Store) SetTime(ctx, key string, t time.Time) error

type Product struct {
    ID int64; Name string; Paused bool
    DropPct *float64; WindowDays, MinHistoryDays *int   // nil = use global
    LastAlertedCents *int64
    CreatedAt time.Time
}
type LinkStatus string // "ok" | "broken" | "pending"
type Link struct {
    ID, ProductID int64; URL, Site, Title, Seller string
    Status LinkStatus; LastError string; LastCheckedAt *time.Time
}
type Variant struct {
    ID, LinkID int64; ExternalID, Name string
    Tracked, IsNew, Gone bool
}
type SessionStatus string // "ok" | "expired" | "blocked" | "unknown"

func (s *Store) CreateProduct(ctx, name string) (Product, error)
func (s *Store) GetProduct(ctx, id int64) (Product, error)          // ErrNotFound
func (s *Store) ListProducts(ctx) ([]Product, error)
func (s *Store) UpdateProduct(ctx, Product) error
func (s *Store) DeleteProduct(ctx, id int64) error                  // cascades
func (s *Store) CreateLink(ctx, productID int64, url, site, title, seller string) (Link, error) // ErrDuplicate if same product+url
func (s *Store) ListLinks(ctx, productID int64) ([]Link, error)
func (s *Store) GetLink(ctx, id int64) (Link, error)
func (s *Store) SetLinkResult(ctx, linkID int64, status LinkStatus, errMsg string, at time.Time) error
func (s *Store) DeleteLink(ctx, id int64) error
func (s *Store) UpsertVariant(ctx, Variant) (Variant, error)        // keyed by (link_id, external_id)
func (s *Store) ListVariants(ctx, linkID int64) ([]Variant, error)
func (s *Store) SetVariantTracked(ctx, id int64, tracked bool) error // also clears IsNew
func (s *Store) GetSession(ctx, site string) (SessionStatus, error)  // "unknown" if unset
func (s *Store) SetSession(ctx, site string, st SessionStatus) error

type Reading struct {
    VariantID int64; At time.Time
    PriceCents, BestCents int64; OriginalCents *int64
    BestIsEstimate, InStock bool
}
func (s *Store) AddReading(ctx, Reading) error
func (s *Store) LogCheck(ctx, linkID int64, at time.Time, ok bool, errMsg string) error
func (s *Store) DailyCheapest(ctx, productID int64, from, to time.Time) ([]alert.DayPrice, error)
func (s *Store) LatestReadings(ctx, productID int64) (map[int64]Reading, error) // by variant id
func (s *Store) History(ctx, productID int64, since time.Time) (map[int64][]Reading, error)
func (s *Store) FirstReadingAt(ctx, productID int64) (time.Time, bool, error)

func (s *Store) NoticeActive(ctx, key string) (bool, error)
func (s *Store) SetNotice(ctx, key string, active bool, at time.Time) error
```
`DailyCheapest` counts only readings of variants that are currently `tracked AND NOT gone`, with `in_stock = 1`, `at >= from AND at < to`, grouped by `date(at + 28800, 'unixepoch')`, returning `MIN(best_cents)` per day ordered by day. Store `at` as unix seconds.

- [ ] **Step 1: Write the failing tests** in `store_test.go`:
  - `TestSettingsDefaults`: fresh store returns `DefaultSettings()`. After `PutSettings` with DropPct 15, read back 15.
  - `TestLinkDuplicate`: second `CreateLink` with the same product and URL returns `ErrDuplicate`.
  - `TestUpsertVariantKeepsTracked`: insert tracked variant, upsert same external id with `Gone: true`, `Tracked` stays true.
  - `TestDailyCheapestSingaporeDay`: readings at `2026-10-17T23:30:00Z` (07:30 SGT on the 18th) 30000 and `2026-10-18T03:00:00Z` 28000 produce one day `"2026-10-18"` with 28000.
  - `TestDailyCheapestFilters`: an out of stock reading of 10000, a reading of an untracked variant of 10000, and a tracked in stock 25000 on the same day produce 25000.
  - `TestNoticeRoundTrip`: `SetNotice("stale", true)` then `NoticeActive` is true. Set false, then false.
  - `TestDeleteProductCascades`: delete removes links, variants and readings.
- [ ] **Step 2:** Run `go test ./internal/store/`. Expected FAIL.
- [ ] **Step 3:** Implement. Schema tables: `settings(key PK, value)`, `products`, `links`, `variants` (unique `link_id, external_id`), `readings(variant_id, at, price_cents, best_cents, original_cents NULL, best_is_estimate, in_stock)` with index `(variant_id, at)`, `check_log`, `sessions(site PK, status, updated_at)`, `notices(key PK, active, sent_at)`. Foreign keys `ON DELETE CASCADE`. Settings stored as JSON under key `settings`.
- [ ] **Step 4:** Run `go test ./internal/store/`. Expected PASS.
- [ ] **Step 5:** Commit `feat(store): sqlite store`.

### Task 3: Alert rule

**Files:**
- Create: `internal/alert/alert.go`, `internal/alert/alert_test.go`

**Interfaces:**
- Produces:
```go
var SGT = time.FixedZone("SGT", 8*3600)
type DayPrice struct { Day string /* "2006-01-02" in SGT */; Cents int64 }
type Params struct { DropPct float64; WindowDays, MinHistoryDays int }
type Input struct {
    Now time.Time
    CurrentCents *int64        // nil when nothing readable
    Daily []DayPrice           // prior days; Evaluate ignores today and days outside the window
    FirstDay string            // SGT day of first successful reading; "" if none
    Params Params
    LastAlertedCents *int64
}
type Result struct {
    Alert bool
    HasTypical bool; TypicalCents int64
    DropPct float64            // (typical-current)/typical*100
    HistoryDays int            // days from FirstDay to today
    LastAlertedCents *int64    // value to store after this check
}
func Evaluate(in Input) Result
func Median(xs []int64) int64 // even count: mean of the middle two, rounded half up
```

Rules, in order:
1. `today` = `Now` in SGT. Window = days `today-WindowDays` through `today-1` inclusive.
2. `HasTypical` if at least one window day exists. `TypicalCents = Median(window values)`.
3. `HistoryDays` = whole days from `FirstDay` to `today`.
4. If `CurrentCents == nil` or `!HasTypical`: no alert, `LastAlertedCents` unchanged.
5. `triggered` = `DropPct >= Params.DropPct`.
6. If `!triggered`: no alert, `LastAlertedCents = nil` (reset).
7. If `triggered` and `HistoryDays < MinHistoryDays`: no alert, unchanged.
8. If `triggered` and `LastAlertedCents != nil` and `current >= *LastAlertedCents`: no alert, unchanged.
9. Otherwise alert, `LastAlertedCents = current`.

- [ ] **Step 1: Write the failing tests** (Now = `2026-10-18T10:00:00+08:00`, defaults 20/30/2 unless stated):
  - `TestMedian`: `[3,1,2]→2`, `[1,2,3,4]→3` (2.5 rounds half up), `[279]→279`.
  - `TestSpecWorkedExample`: daily all 27900, current 22900, DropPct param 15 → Alert, `DropPct≈17.92`. Same with param 20 → no alert.
  - `TestSlowSlide`: 25 days at 10000 then 95, 90, 85, 80 (×100) for the last 4 days, current 7500, param 25 → Alert, typical 10000.
  - `TestFakeSpikeIgnored`: 28 days at 10000, 2 days at 15000, current 10500, param 20 → no alert, typical 10000.
  - `TestTodayExcluded`: a daily entry for today at 1000 does not change the typical.
  - `TestMinHistory`: FirstDay `2026-10-17` (1 day) and a triggered price → no alert. FirstDay `2026-10-16` (2 days) → alert.
  - `TestNoSpam`: LastAlerted 20000, current 20000 → no alert. Current 19900 → alert and LastAlerted 19900.
  - `TestReset`: LastAlerted 20000, current back at typical → no alert, LastAlerted nil.
  - `TestNoCurrentKeepsState`: CurrentCents nil, LastAlerted 20000 → no alert, LastAlerted still 20000.
  - `TestWindowEdge`: a day exactly `WindowDays+1` ago at 1 cent is ignored.
- [ ] **Step 2:** Run `go test ./internal/alert/`. Expected FAIL.
- [ ] **Step 3:** Implement per the rules above.
- [ ] **Step 4:** Run `go test ./internal/alert/`. Expected PASS.
- [ ] **Step 5:** Commit `feat(alert): typical price drop rule`.

### Task 4: Reader contract and browser

**Files:**
- Create: `internal/reader/reader.go`, `internal/reader/reader_test.go`, `internal/reader/jsonld.go`, `internal/reader/jsonld_test.go`, `internal/browser/chrome.go`, `cmd/capture/main.go`

**Interfaces:**
- Produces:
```go
// package reader
type Site string
const (Shopee Site = "shopee"; Lazada = "lazada"; Amazon = "amazon"; Courts = "courts")
type VariantPrice struct {
    ExternalID, Name string
    PriceCents, BestCents int64
    OriginalCents int64        // 0 if none shown
    BestIsEstimate, InStock bool
}
type Listing struct { Site Site; URL, Title, Seller string; Variants []VariantPrice }
var (ErrLoggedOut = errors.New("logged out"); ErrBlocked = errors.New("blocked by verification"); ErrUnreadable = errors.New("price not found"))
type Page struct { FinalURL, HTML string; Responses map[string][]byte } // keyed by response URL
type Browser interface { Load(ctx context.Context, url string, capture []string) (Page, error) }
type Reader interface {
    Site() Site
    Canonical(raw string) (string, bool)   // false if this reader doesn't handle the URL
    Read(ctx context.Context, b Browser, url string, want []string) (Listing, error)
}
func Register(r Reader)
func For(raw string) (Reader, string, error) // reader and canonical URL; error "unsupported site"
func Validate(l Listing) error               // ErrUnreadable if no variants or any BestCents/PriceCents <= 0, or BestCents > PriceCents
func ProductFromJSONLD(html string) (name string, priceCents int64, inStock bool, ok bool)

// package browser
type Config struct { ChromePath, ProfileDir string; Settle time.Duration } // Settle default 6s
func New(cfg Config) *Chrome
func (c *Chrome) Load(ctx, url string, capture []string) (reader.Page, error) // offscreen
func (c *Chrome) OpenVisible(ctx, url string, done func(currentURL string) bool, timeout time.Duration) error
```
`want` lists variant external IDs to price exactly. Empty means every variant the page shows, at whatever precision it gives.

`Chrome` holds a mutex so only one Load or OpenVisible runs at once. Each call launches Chrome with the Global Constraints flags, and closes it at the end. OpenVisible omits `window-position`, polls the URL every 2s, and returns when `done` is true or the timeout passes. Load navigates, waits `Settle`, then reads `FinalURL`, `document.documentElement.outerHTML`, and the bodies of responses whose URL contains any `capture` substring (listen to `network.EventResponseReceived`, fetch bodies with `network.GetResponseBody` after the settle). Reuse the working calls from the spike at `/private/tmp/claude-501/-Users-lersonv-Documents-Projects-golang-dropcatch/4b219340-c12b-4820-858d-b198c371847e/scratchpad/spike/main.go` if it still exists.

`cmd/capture`: `go run ./cmd/capture -profile <dir> -out <dir> [-capture substr,...] [-login url] <url>`. With `-login`, opens visible until you close it (10 min cap). Otherwise loads the URL and writes `page.html`, `final_url.txt`, and one file per captured response.

- [ ] **Step 1: Write the failing tests**: `TestValidate` rejects an empty listing, a zero `BestCents`, and `BestCents > PriceCents`, and accepts a good one. `TestForUnsupported` returns an error for `https://example.com/x`. `TestProductFromJSONLD` on an inline HTML string with `{"@type":"Product","name":"X","offers":{"price":"1299.00","availability":"https://schema.org/InStock"}}` returns `"X", 129900, true, true`. Also handle `offers` as an array and `@graph` wrappers.
- [ ] **Step 2:** Run `go test ./internal/reader/`. Expected FAIL.
- [ ] **Step 3:** Implement `reader.go`, `jsonld.go`, then `browser/chrome.go` and `cmd/capture`. `go get github.com/chromedp/chromedp@v0.14.2` (v0.21 has a different generic API).
- [ ] **Step 4:** Run `go test ./internal/reader/ && go build ./...`. Expected PASS.
- [ ] **Step 5: Manual check**: `go run ./cmd/capture -profile /tmp/dc-profile -login https://shopee.sg/buyer/login https://shopee.sg`, log in, close. Then `go run ./cmd/capture -profile /tmp/dc-profile -out /tmp/cap -capture /api/v4/pdp/get_pc https://shopee.sg/product/442800909/40818293338`. Expected `/tmp/cap` holds a `get_pc` JSON with `"Apple AirPods Pro 3"`.
- [ ] **Step 6:** Commit `feat(reader): reader contract, chrome browser, capture tool`.

### Task 5: Shopee reader

**Files:**
- Create: `internal/reader/shopee/shopee.go`, `shopee_test.go`, `testdata/get_pc_airpods.json`

**Interfaces:**
- Consumes: Task 4 `reader` types.
- Produces: `shopee.New() reader.Reader`, plus pure helpers `ParseIDs(raw string) (shopID, itemID int64, ok bool)` and `ParseGetPC(body []byte, wantModel string) (reader.Listing, error)`.

Facts from the spike (`get_pc` response, values in 1/100000 SGD):
- `data.item.title`, `data.shop_detailed.name` (seller), `data.item.models[]` with `model_id`, `name`, `price`, `price_before_discount`, `status` (1 = normal), `data.item.stock_display` (`"IN STOCK"`).
- `data.product_price.price.single_value` is the after voucher price, valid when `data.product_price.has_final_price` is true, and it belongs to model `data.product_price.final_price_info.model_id`.
- Logged out lands on `/verify/traffic/error` with "Login Required". A captcha lands on a `/verify/` URL too. Tell them apart by `is_logged_in=false` in the URL query.

Rules:
- Canonical URL `https://shopee.sg/product/<shop>/<item>`. Accept `-i.<shop>.<item>` slugs, `/product/<shop>/<item>`, any query string. For `s.shopee.sg` short links return the raw URL as canonical with ok true, and `Read`/preview re-canonicalize from `Page.FinalURL`.
- `ParseGetPC`: each model gives a `VariantPrice` with `ExternalID = strconv(model_id)`, `PriceCents = price/1000`, `OriginalCents = price_before_discount/1000` (0 if equal to price), `InStock = status == 1 && stock_display != "SOLD OUT"`. `BestCents = single_value/1000` only for the model matching `final_price_info.model_id` when `has_final_price`. Every other model gets `BestCents = PriceCents` and `BestIsEstimate = true`. `wantModel` is unused for matching. The voucher model id in the data decides.
- `Read(want)`: load the canonical URL with `?extraParams={"display_model_id":<id>}` (URL encoded) once per wanted model, capturing `/api/v4/pdp/get_pc`, waiting a random 5 to 20 seconds between loads (inject the sleeper for tests). Merge, keeping for each model the reading where it got the voucher price. With empty `want`, load once without the param. A `FinalURL` containing `/verify/` returns `ErrLoggedOut` if `is_logged_in=false`, else `ErrBlocked`. A missing `get_pc` response returns `ErrUnreadable`. Finish with `reader.Validate`.

- [ ] **Step 1:** Copy the spike capture: `cp /private/tmp/claude-501/-Users-lersonv-Documents-Projects-golang-dropcatch/4b219340-c12b-4820-858d-b198c371847e/scratchpad/spike/get_pc.json internal/reader/shopee/testdata/get_pc_airpods.json`. If it's gone, capture it with Task 4's tool.
- [ ] **Step 2: Write the failing tests**:
  - `TestParseIDs`: `https://shopee.sg/Apple-AirPods-Pro-3-i.442800909.40818293338?extraParams=x` → 442800909, 40818293338. `https://shopee.sg/product/442800909/40818293338` → same. `https://shopee.sg/` → not ok.
  - `TestParseGetPCAirpods`: title `"Apple AirPods Pro 3"`, seller `"Apple Flagship Store"`, one variant `ExternalID "261503633746"`, `Name "Pro 3"`, `PriceCents 31600`, `BestCents 31500`, `OriginalCents 34900`, `InStock true`, `BestIsEstimate false`.
  - `TestParseGetPCVoucherForOtherModel`: mutate the fixture in the test so `final_price_info.model_id` is `999`. The Pro 3 variant gets `BestCents 31600` and `BestIsEstimate true`.
  - `TestParseGetPCZeroPrice`: mutate `price` to 0. `reader.Validate` on the result returns `ErrUnreadable`.
  - `TestReadDetectsLogin` and `TestReadDetectsBlock` with a fake `Browser` returning `FinalURL` `https://shopee.sg/verify/traffic/error?is_logged_in=false` and `https://shopee.sg/verify/captcha?x=1`.
- [ ] **Step 3:** Run `go test ./internal/reader/shopee/`. Expected FAIL.
- [ ] **Step 4:** Implement. Register in an `init` or in `main` wiring (pick `main`, keeping registration explicit).
- [ ] **Step 5:** Run `go test ./internal/reader/shopee/`. Expected PASS.
- [ ] **Step 6: Manual check on a multi variant listing.** Capture any Shopee listing with 2+ variants twice, with two different `display_model_id` values. Check whether `final_price_info.model_id` follows the param. Write the answer as a comment at the top of `shopee.go`. If it doesn't follow, drop the per-model reload so `Read` loads once.
- [ ] **Step 7:** Commit `feat(reader): shopee`.

### Task 6: Lazada reader

**Files:**
- Create: `internal/reader/lazada/lazada.go`, `lazada_test.go`, `testdata/`

**Interfaces:**
- Produces: `lazada.New() reader.Reader`, `ParseIDs(raw) (itemID, skuID string, ok bool)`, `ParsePage(p reader.Page) (reader.Listing, error)`.

Lazada product URLs look like `https://www.lazada.sg/products/<slug>-i<itemId>-s<skuId>.html`. Canonical drops the query string. Variant `ExternalID` is the sku id. Login URL for sessions is `https://member.lazada.sg/user/login`.

- [ ] **Step 1: Capture.** Log in to Lazada in the capture profile. Capture one AirPods Pro 3 listing with 2+ variants with `-capture pdp,mtop`. Save to `testdata/`. Find where the page keeps sku prices, the voucher or "best price" figure, the seller name and stock (embedded JSON in a script tag is likely). Also capture once logged out and once from a fresh profile to see what a block page looks like. Write what you found as a comment at the top of `lazada.go`.
- [ ] **Step 2: Write the failing tests** with values read from your capture: title, seller, each sku's id, name, price, best price, stock. `TestParseIDs` on the URL shape above. Logged out page → `ErrLoggedOut`. Block page → `ErrBlocked`.
- [ ] **Step 3:** Run `go test ./internal/reader/lazada/`. Expected FAIL.
- [ ] **Step 4:** Implement. If Lazada shows no voucher price, `BestCents = PriceCents` with `BestIsEstimate = false` (nothing was estimated, there's simply no voucher).
- [ ] **Step 5:** Run tests. Expected PASS.
- [ ] **Step 6:** Commit `feat(reader): lazada`.

### Task 7: Amazon SG and Courts readers

**Files:**
- Create: `internal/reader/amazon/amazon.go`, `amazon_test.go`, `testdata/`, `internal/reader/courts/courts.go`, `courts_test.go`, `testdata/`

**Interfaces:**
- Produces: `amazon.New()`, `amazon.ParseHTML(html string) (reader.Listing, error)`, `amazon.Coupon(text string, priceCents int64) int64` (returns cents off). `courts.New()`, `courts.ParseHTML(html string) (reader.Listing, error)`.

Amazon: canonical `https://www.amazon.sg/dp/<ASIN>`, one variant per URL with `ExternalID` = ASIN. Price from the first `#corePrice_feature_div .a-offscreen` (fall back to `#corePriceDisplay_desktop_feature_div .a-offscreen`). Original from `.basisPrice .a-offscreen`. Coupon text from elements whose text contains `coupon` within `#promoPriceBlockMessage_feature_div` or `#couponBadge*`. Stock: `#availability` text contains "in stock" (case insensitive). A page with "Enter the characters you see below" → `ErrBlocked`. No login.

Courts: canonical drops the query. One variant, `ExternalID` = the URL path. Use `reader.ProductFromJSONLD`. Fall back to a visible price selector only if the capture shows no JSON-LD.

- [ ] **Step 1: Capture** one Amazon SG AirPods page (ideally one with a coupon) and one Courts AirPods page into each `testdata/`. Adjust selectors above to what the capture shows.
- [ ] **Step 2: Write the failing tests**: `TestCoupon`: `"Apply S$20.00 coupon"` on 27500 → 2000, `"Apply 10% coupon"` on 27500 → 2750, `"Save 5% on 2 items"` → 0. `TestParseHTML` for each site with values from the capture. `TestAmazonBlocked` on a minimal HTML containing the captcha sentence.
- [ ] **Step 3:** Run `go test ./internal/reader/...`. Expected FAIL.
- [ ] **Step 4:** Implement with goquery. Amazon `BestCents = PriceCents - Coupon`.
- [ ] **Step 5:** Run tests. Expected PASS.
- [ ] **Step 6:** Commit `feat(reader): amazon sg and courts`.

### Task 8: Email

**Files:**
- Create: `internal/notify/resend.go`, `emails.go`, `notices.go`, `notify_test.go`

**Interfaces:**
- Consumes: `store.Store` notice methods, `money.Format`.
- Produces:
```go
type Mailer interface { Send(ctx context.Context, to, subject, text string) error }
type Resend struct { APIKey, BaseURL string; HTTP *http.Client } // BaseURL default https://api.resend.com
type DropAlert struct {
    Product, Site, Seller, Variant, URL string
    NewCents, TypicalCents int64; DropPct float64
    HistoryDays, WindowDays int
}
func DropEmail(a DropAlert) (subject, text string)
func SessionEmail(site string, blocked bool) (subject, text string)
func BrokenLinkEmail(product, url, errMsg string) (subject, text string)
func StaleEmail(lastSuccess time.Time) (subject, text string)
func TestEmail() (subject, text string)
type Notices struct { Store *store.Store; Mailer Mailer; To func(ctx) (string, error); Now func() time.Time }
func (n *Notices) Raise(ctx, key, subject, text string) error // sends only when the key isn't already active
func (n *Notices) Clear(ctx, key string) error
```
Notice keys: `session:<site>`, `link:<id>`, `stale`.

Copy:
- Drop subject: `"<Product> dropped to <$new> (<pct>% under typical)"`, pct rounded to a whole number.
- Drop body lines: product, `<Site> · <Seller> · <Variant>`, new price, typical price over the last N days, percent drop, the URL. When `HistoryDays < WindowDays`, add `"Based on <HistoryDays> days of history."`
- Session body says to open Dropcatch Settings and click the log in button for that site.

- [ ] **Step 1: Write the failing tests**:
  - `TestResendRequest` against `httptest.Server`: method POST, path `/emails`, header `Authorization: Bearer k`, JSON `from` `"Dropcatch <onboarding@resend.dev>"`, `to` `["me@x.com"]`. A 4xx response returns an error containing the body.
  - `TestDropEmail`: AirPods, 22900 vs 27900, 17.92%, 3 of 30 days → subject `"AirPods Pro 3 dropped to $229.00 (18% under typical)"`, body contains `"$279.00"`, the URL, and `"Based on 3 days of history."`
  - `TestNoticesOnce`: Raise twice sends one email. Clear then Raise sends again.
- [ ] **Step 2:** Run `go test ./internal/notify/`. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run tests. Expected PASS.
- [ ] **Step 5:** Commit `feat(notify): resend email and one-shot notices`.

### Task 9: Checker

**Files:**
- Create: `internal/checker/checker.go`, `checker_test.go`

**Interfaces:**
- Consumes: store, alert, reader, notify.
- Produces:
```go
type Checker struct {
    Store *store.Store; Browser reader.Browser
    Mailer notify.Mailer; Notices *notify.Notices
    Now func() time.Time; Sleep func(time.Duration)  // random 5–20s pause between links
}
func (c *Checker) Preview(ctx context.Context, rawURL string) (reader.Listing, error)
func (c *Checker) CheckProduct(ctx context.Context, productID int64) error
func (c *Checker) RunAll(ctx context.Context) error // non-paused products in id order
```

`CheckProduct`:
1. For each link: if the site session is `expired` or `blocked`, skip it (no read, no log). Otherwise `reader.For(link.URL)`, then `Read` with `want` = external IDs of tracked, non-gone variants.
2. On `ErrLoggedOut`/`ErrBlocked`: set session `expired`/`blocked`, `LogCheck` fail, Raise `session:<site>`. Leave the link status alone (it isn't the link's fault).
3. On other errors: link `broken` with the message, `LogCheck` fail, Raise `link:<id>` with `BrokenLinkEmail`.
4. On success: link `ok`, Clear `link:<id>`, session `ok`, Clear `session:<site>`, set `last_success_at`. Reconcile variants: unknown external id → insert `Tracked false, IsNew true`. Known but absent → `Gone true`. Present → `Gone false`, update name. Add a `Reading` for every present variant that is tracked.
5. After all links: `current` = min `BestCents` over this run's new readings that are tracked and in stock (nil if none). Build `alert.Input` from `DailyCheapest(from = today-WindowDays, to = start of today SGT)`, `FirstReadingAt`, effective params (product override else global), and `Product.LastAlertedCents`. On `Alert`, send `DropEmail` for the cheapest reading's variant and link via `Mailer` directly (not Notices). Save the returned `LastAlertedCents`.
6. Sleep between links, not after the last.

`Preview`: `reader.For`, `Read` with empty `want`, return the listing with `URL` set to the canonical URL (re-canonicalized from the page for short links). On `ErrLoggedOut`/`ErrBlocked`, also update the session.

- [ ] **Step 1: Write the failing tests** with `:memory:` store, a fake reader registered for `https://fake.test/...`, a fake `Browser`, a recording `Mailer`, `Sleep` no-op, and fixed `Now`:
  - `TestCheckRecordsReadings`: one tracked variant at 25000 → one reading, link ok.
  - `TestNewVariantUntracked`: page shows a second variant → stored `Tracked false, IsNew true`, no reading for it.
  - `TestGoneVariant`: tracked variant missing from the page → `Gone true`, history kept.
  - `TestBrokenLinkEmailsOnce`: reader returns `ErrUnreadable` twice → link broken, one email.
  - `TestSessionExpiredSkipsSite`: first run `ErrLoggedOut` → one email, session expired. Second run doesn't call the reader for that site.
  - `TestDropAlertSends`: seed 5 days of readings at 27900, FirstDay 5 days ago, product DropPct 15, page now 22900 → one drop email with subject from Task 8, `LastAlertedCents 22900`.
  - `TestAllLinksFailNoReset`: product with `LastAlertedCents 20000`, every read fails → no drop email, `LastAlertedCents` still 20000.
  - `TestOutOfStockIgnored`: cheapest variant at 10000 out of stock, other at 27900 → no alert.
- [ ] **Step 2:** Run `go test ./internal/checker/`. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run tests. Expected PASS.
- [ ] **Step 5:** Commit `feat(checker): check products, reconcile variants, alert`.

### Task 10: Scheduler and stale check

**Files:**
- Create: `internal/scheduler/scheduler.go`, `scheduler_test.go`

**Interfaces:**
- Consumes: `checker.Checker`, store times, `notify.Notices`.
- Produces:
```go
func Due(lastRun time.Time, interval time.Duration, now time.Time) bool // zero lastRun → true
func Stale(lastSuccess time.Time, hasTracked bool, now time.Time, limit time.Duration) bool
type Scheduler struct { Checker *checker.Checker; Store *store.Store; Notices *notify.Notices; Now func() time.Time; Tick time.Duration } // Tick 1m
func (s *Scheduler) Run(ctx context.Context)
func (s *Scheduler) TickOnce(ctx context.Context)
```
`TickOnce`: (a) stale check first. If `Stale(last_success_at, any tracked variant exists, now, 12h)` Raise `stale` with `StaleEmail`, else Clear `stale`. A zero `last_success_at` with tracked variants counts from the first product's `CreatedAt`, so a brand new install doesn't email at once. (b) If `Due(last_run_at, interval, now)`: set `last_run_at = now` first, then `RunAll`. `Run` calls `TickOnce` at start, then on each tick. Because `Due` compares wall clock time, a laptop that slept past the interval runs on the first tick after wake.

- [ ] **Step 1: Write the failing tests**: `TestDue` (zero → true, 2h59m → false, 3h → true, 9h after sleep → true). `TestStale` (11h59m → false, 12h → true, no tracked → false). `TestTickOnceRunsOnceAfterWake`: last run 9h ago, two `TickOnce` calls one minute apart → `RunAll` runs once (use a fake reader that counts calls).
- [ ] **Step 2:** Run `go test ./internal/scheduler/`. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run tests. Expected PASS.
- [ ] **Step 5:** Commit `feat(scheduler): interval runs, wake catch up, stale email`.

### Task 11: HTTP API

**Files:**
- Create: `internal/server/server.go`, `products.go`, `settings.go`, `server_test.go`

**Interfaces:**
- Consumes: store, checker, alert, notify, `browser.Chrome.OpenVisible` (via an interface `LoginOpener { OpenVisible(...) error }`).
- Produces: `server.New(deps Deps, static fs.FS) http.Handler`. JSON in camelCase, money as integer cents, times RFC3339. Errors are `{"message": "..."}` with 400, 404, 409 or 500 so the template's `ApiError` picks up the message.

Routes:
| Method and path | Body | Returns |
|---|---|---|
| `GET /api/products` | | `ProductSummary[]` |
| `POST /api/products` | `{name, links:[{url, variantIds}]}` | `ProductDetail` 201 |
| `GET /api/products/{id}` | | `ProductDetail` |
| `PATCH /api/products/{id}` | `{name?, paused?, dropPct?, windowDays?, minHistoryDays?}`, explicit `null` clears an override | `ProductDetail` |
| `DELETE /api/products/{id}` | | 204 |
| `GET /api/products/{id}/history?range=7d\|30d\|90d\|all` | | `{series:[{variantId,label,points:[{t,cents}]}], typicalCents}` |
| `POST /api/products/{id}/check` | | 202, runs `CheckProduct` in a goroutine |
| `POST /api/products/{id}/links` | `{url, variantIds}` | `ProductDetail` 201 |
| `DELETE /api/links/{id}` | | 204 |
| `PATCH /api/variants/{id}` | `{tracked}` | 204 |
| `POST /api/preview` | `{url}` | `{site,url,title,seller,variants:[{externalId,name,priceCents,bestCents,inStock}]}` |
| `GET /api/settings` | | settings with `resendApiKey` masked to its last 4 chars |
| `PUT /api/settings` | settings; a masked key value leaves the stored key unchanged | settings |
| `POST /api/settings/test-email` | | 204 |
| `GET /api/sessions` | | `[{site, status}]` for shopee and lazada |
| `POST /api/sessions/{site}/login` | | 202. In a goroutine, calls `OpenVisible` at the site's login URL with a `done` that never returns true, so it ends when you close the window or after 10 minutes. Then sets the session to `unknown` so checks resume. A second call while one is open returns 409 |

`ProductSummary`: `id, name, paused, linkCount, bestCents|null, bestSite, typicalCents|null, pctVsTypical|null, spark:[{day,cents}] (30 days), warnings:[string]` where warnings include `"<n> link(s) broken"`, `"<site> login expired"`, `"<site> needs verification"`, `"<n> new variant(s)"`. Best now = min of each tracked, non-gone variant's latest reading that is in stock. Typical comes from `alert.Evaluate` with the same inputs the checker uses.

`ProductDetail`: summary fields plus `settings: {dropPct, windowDays, minHistoryDays}` (effective), `overrides` (the nullable ones), and `links:[{id,url,site,title,seller,status,lastError,lastCheckedAt, variants:[{id,externalId,name,tracked,isNew,gone,inStock,priceCents,bestCents,bestIsEstimate,originalCents}]}]`.

`POST /api/products` and `/links`: for each link, run `Preview` (this hits the browser), canonicalize, create the link and every variant from the preview with `Tracked = externalId in variantIds`, `IsNew false`. A duplicate canonical URL in the same product returns 409 `"This link is already tracked"`.

Anything outside `/api/` serves from `static`. Unknown paths fall back to `index.html`.

- [ ] **Step 1: Write the failing tests** with `httptest`, `:memory:` store and the fake reader from Task 9: create a product with one link and one of two variants ticked → detail shows 2 variants, 1 tracked. `PATCH` with `{"dropPct": null}` clears the override. Same link added twice in two URL shapes → 409. `GET /api/settings` masks the key. `PUT` with the masked value keeps the original. `GET /nope` returns `index.html`.
- [ ] **Step 2:** Run `go test ./internal/server/`. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run tests. Expected PASS.
- [ ] **Step 5:** Commit `feat(server): json api`.

### Task 12: Frontend setup

**Files:**
- Create: `web/` (template copy), `web/embed.go`, `web/src/lib/api/products.ts`, `web/src/lib/api/settings.ts`
- Modify: `web/vite.config.ts`, `web/src/router.tsx`, `web/src/app/layout/app-sidebar.tsx`, `web/src/app/layout/breadcrumbs.tsx`, `web/package.json`

**Interfaces:**
- Produces: TypeScript types mirroring Task 11 JSON (`ProductSummary`, `ProductDetail`, `LinkDetail`, `VariantDetail`, `History`, `Preview`, `Settings`, `Session`) and query option factories and mutations:
  - `products.ts`: `productKeys`, `productsQueryOptions()`, `productQueryOptions(id)`, `historyQueryOptions(id, range)`, `useCreateProduct()`, `useUpdateProduct()`, `useDeleteProduct()`, `useAddLink()`, `useDeleteLink()`, `useSetVariantTracked()`, `useCheckNow()`, `usePreviewLink()`.
  - `settings.ts`: `settingsKeys`, `settingsQueryOptions()`, `sessionsQueryOptions()`, `useSaveSettings()`, `useSendTestEmail()`, `useOpenLogin()`.
- `formatCents(cents: number): string` in `web/src/lib/format.ts`.

- [ ] **Step 1:** `git clone --depth 1 https://github.com/vuhoang38/vibecode-template web && rm -rf web/.git`. Run the README's "Delete the examples" commands inside `web/`, and clear their route, nav and breadcrumb entries. Rename the package to `dropcatch-web`.
- [ ] **Step 2:** `vite.config.ts`: add `server.proxy: { '/api': 'http://127.0.0.1:8080' }`. Leave `VITE_API_URL` unset.
- [ ] **Step 3:** Write `web/embed.go`: `package web` with `//go:embed all:dist` `var Dist embed.FS`, and add `web/dist/.gitkeep`.
- [ ] **Step 4: Write the failing test** `web/src/lib/format.test.ts`: `formatCents(129900) === "$1,299.00"`.
- [ ] **Step 5:** Implement `format.ts`, `products.ts` and `settings.ts` with `api.get/post/patch/put/delete` from `@/lib/api/client`. Mutations invalidate `productKeys.all` (or `settingsKeys.all`).
- [ ] **Step 6:** Routes in `router.tsx`: index → `@/routes/products/page`, `products/:id` → `@/routes/product/page`, `settings` → `@/routes/settings/page`. Sidebar: Products, Settings. Create placeholder pages that render a `PageHeader` so the build passes.
- [ ] **Step 7:** Run `cd web && bun install && bun run verify`. Expected clean.
- [ ] **Step 8:** Commit `feat(web): dashboard shell from template`.

### Task 13: Products page and add product

**Files:**
- Create: `web/src/routes/products/page.tsx`, `page.test.tsx`, `components/product-row.tsx`, `components/sparkline.tsx`, `components/add-product-dialog.tsx`, `components/link-variant-picker.tsx`, `components/link-variant-picker.test.tsx`

**Interfaces:**
- Consumes: Task 12 queries and mutations.
- Produces: `LinkVariantPicker` props `{ onPicked(link: { url: string; variantIds: string[]; preview: Preview }): void }`. Task 14 reuses it to add a link, so put it in `web/src/components/shared/` instead of the route folder.

Behavior (Layout A from the spec):
- Rows: name, link count, sparkline of `spark`, best price with site, and `"<n>% under typical"` / `"at typical"` / `"<n>% over typical"`. Warnings as a badge. Row click goes to `/products/:id`. Empty state with the Add button.
- Add dialog: name field, then one or more links. For each link, a URL input and a "Read" button that calls `usePreviewLink`. While it loads, show that it can take up to a minute. It then shows title, seller and a checklist of variants with price and best price. Buttons "Tick all" and "Untick all". A single variant listing comes pre ticked. Save is disabled until the name is set and every link has at least one ticked variant. Preview errors show the server message inline (for example a Shopee login expired message).
- Use `recipes/add-a-dialog-or-sheet.md`, `add-a-form.md` and `add-a-chart.md` patterns.

- [ ] **Step 1: Write the failing tests**: products page renders a row with `"18% under typical"` for best 22900 and typical 27900 (mock `@/lib/api/client`). `LinkVariantPicker` after a mocked preview with 2 variants shows 2 checkboxes, "Tick all" ticks both, `onPicked` gets both ids. A 1 variant preview comes pre ticked.
- [ ] **Step 2:** Run `cd web && bun run test`. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `bun run verify` and `bun run screenshot /` and `bun run screenshot --dark /`. Open the PNGs. Expected clean, and the page looks like Layout A.
- [ ] **Step 5:** Commit `feat(web): products list and add product`.

### Task 14: Product page

**Files:**
- Create: `web/src/routes/product/page.tsx`, `page.test.tsx`, `components/price-chart.tsx`, `components/links-table.tsx`, `components/alert-settings-form.tsx`

**Interfaces:**
- Consumes: Task 12 queries, `LinkVariantPicker` from Task 13.

Behavior:
- Header: name, Pause/Resume, "Check now" (toast "Checking, this takes a minute"), Delete with confirm dialog.
- Three stats: best now with site, typical price, alert threshold (`"<dropPct>% under"`).
- Chart: Recharts line per series, dashed horizontal line at `typicalCents`, range toggle 7d / 30d / 90d / All, default 30d. Use the chart recipe and colors from tokens.
- Links table: one row per tracked variant with site, seller, variant, now price, best price (mark estimates with a tooltip "Voucher price not shown for this variant"), last checked, status. Under each link, an expandable list of all its variants with checkboxes calling `useSetVariantTracked`, a "New" badge on `isNew`, and "Gone" on `gone`. "Add link" opens `LinkVariantPicker`. Remove link with confirm.
- Alert settings form: drop %, window days, minimum history days, each with "use default" to clear the override.

- [ ] **Step 1: Write the failing tests**: renders the three stats from a mocked detail. Unticking a variant calls `PATCH /variants/{id}` with `{tracked:false}`. Clearing the drop override sends `{dropPct:null}`.
- [ ] **Step 2:** Run tests. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `bun run verify`, then screenshot `/products/1` against a running backend with seeded data, light and dark. Open the PNGs.
- [ ] **Step 5:** Commit `feat(web): product page`.

### Task 15: Settings page

**Files:**
- Create: `web/src/routes/settings/page.tsx`, `page.test.tsx`

Behavior: form for drop %, window days, minimum history days, check interval hours, Resend API key (password input showing the masked value), email. "Send test email" button. A sessions card with Shopee and Lazada status badges (`ok`, `expired`, `needs verification`, `unknown`) and a "Log in" button each, which calls `useOpenLogin` and toasts "A Chrome window opened. Log in there, then close it."

- [ ] **Step 1: Write the failing test**: saving with a changed drop % sends `PUT /settings` with the new value and the masked key unchanged.
- [ ] **Step 2:** Run tests. Expected FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `bun run verify`, screenshot `/settings` light and dark, open the PNGs.
- [ ] **Step 5:** Commit `feat(web): settings page`.

### Task 16: Binary, launch agent, end to end

**Files:**
- Create: `cmd/dropcatch/main.go`, `Makefile`, `README.md`

**Interfaces:**
- Consumes: everything.

`main.go`:
- `dropcatch [serve]` flags `-addr 127.0.0.1:8080`, `-data <dir>` (default from Global Constraints), `-chrome <path>`. Creates the data dir, opens the store, registers the four readers, builds Chrome with `ProfileDir = <data>/chrome-profile`, wires `Notices.To` to the settings email, `Sleep` to a random 5 to 20 second pause, starts the scheduler goroutine, serves `server.New(..., fs.Sub(web.Dist, "dist"))`. Logs to stderr. Shuts down cleanly on SIGINT/SIGTERM.
- `dropcatch install-agent`: writes `~/Library/LaunchAgents/com.dropcatch.agent.plist` with `ProgramArguments` = absolute path of the running binary plus `serve`, `RunAtLoad true`, `KeepAlive true`, stdout and stderr to `<data>/logs/dropcatch.log`. Then runs `launchctl bootout gui/<uid>/com.dropcatch.agent` (ignore errors) and `launchctl bootstrap gui/<uid> <plist>`. Prints the dashboard URL.

`Makefile`: `web` (`cd web && bun install && bun run build`), `build` (`web` then `go build -o dropcatch ./cmd/dropcatch`), `test` (`go test ./... && cd web && bun run verify`), `install` (`build` then `./dropcatch install-agent`).

README: what it is, `make install`, first run steps (open `http://127.0.0.1:8080`, Settings, paste the Resend key and your email, send a test email, log in to Shopee and Lazada), and where data and logs live.

- [ ] **Step 1:** Implement `main.go`, `Makefile`, README.
- [ ] **Step 2:** Run `make test`. Expected all Go tests pass and `bun run verify` is clean.
- [ ] **Step 3: End to end by hand.** `make build && ./dropcatch -data /tmp/dc-e2e`. Then:
  - Settings, add the Resend key and email, send a test email. Expected one email arrives.
  - Log in to Shopee from Settings. Expected status turns `unknown`, then `ok` after a check.
  - Add "AirPods Pro 3" with `https://shopee.sg/Apple-AirPods-Pro-3-i.442800909.40818293338`, tick its variant. Click "Check now". Expected a reading near the live after voucher price and a chart point.
  - Add one Lazada, one Amazon SG and one Courts AirPods link to the same product. Expected each reads a price.
  - Set the product's drop % to 1 and its minimum history to 0, then seed one earlier day with `sqlite3 /tmp/dc-e2e/dropcatch.db` at a price 10% higher than now, and click "Check now". Expected one drop email with the subject format from Task 8. Click again. Expected no second email.
  - Point a link at a page that doesn't exist. Expected the link shows broken and one broken link email.
- [ ] **Step 4:** `make install`. Log out and back in to macOS (or `launchctl kickstart`). Expected the dashboard answers at `http://127.0.0.1:8080`.
- [ ] **Step 5:** Commit `feat: dropcatch binary and launch agent`.
