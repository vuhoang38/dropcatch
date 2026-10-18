# Dropcatch design

Date: 2026-10-18

## Why this exists

Shopee always says AirPods Pro 3 are "on sale," but the price never seems to move much. The crossed out "original" price is often made up. You want your own price history so you can tell a real deal from a fake one, and you want an email fast when a price really drops.

Dropcatch is a personal price tracker. It runs on your laptop, checks product pages on Shopee, Lazada, Amazon, and Courts every few hours, keeps the history, charts it, and emails you when a product gets meaningfully cheaper than its usual price.

## Goals

1. Record the real price you'd pay for a product across several sites and listings, on a schedule.
2. Show that history on a dashboard so a fake sale is obvious at a glance.
3. Email you quickly when the cheapest price across all links falls well below the product's typical price.
4. Let you add products, paste links, and pick variants from the dashboard.

## Non goals

- Multiple users, sign up, or app login. This is for one person.
- Hosting on a server. It runs on your laptop.
- Buying anything, adding to cart, or visiting checkout.
- Wallet vouchers that only apply at checkout, and shipping fees.
- Perfect history. A missed check leaves a gap and that is fine.

## Anti goals

These would make it a failure even if it technically works.

- Alerts based on the seller's "was" price. That is exactly the trick this app exists to see through.
- Emails for fake drops, like a cheap "case only" variant or an out of stock price.
- Silent failure. If a login expires or a page stops parsing, you must find out, because a dead tracker misses drops without telling you.
- Getting your Shopee or Lazada account flagged by browsing too aggressively.

## Constraints

- Backend in Go.
- Frontend built from https://github.com/vuhoang38/vibecode-template (Vite, React, TypeScript, React Router, TanStack Query, Tailwind v4, shadcn/ui, Recharts, Bun). Follow its CLAUDE.md, house lint rules, and recipes. Delete its example routes.
- Runs only while the laptop is awake.
- Singapore sites. All prices in SGD.
- Shopee and Lazada need your logged in session to show your vouchers.

## What gets tracked

**Product.** Something you want to buy, like "AirPods Pro 3." It has a name, alert settings, and a set of links. You can pause tracking on a product.

**Link.** One listing URL on one site. When you paste a link, the app opens it once right away and shows the title, seller, variants, and current prices it read, so you catch a wrong link before saving.

**Variants.** A listing may have several variants at different prices. The app shows them as a checklist. Ticked variants get tracked and count toward the alert. Unticked ones are ignored. You can choose one variant, all of them, or any mix, and change the ticks later from the product page. Shopee and Lazada links usually don't say which variant you clicked, so you pick in the app, not in the URL. A listing with no variants is treated as one variant.

New variants that appear later show up unticked with a "new variant" badge (builder's call, chosen so a surprise cheap add on can't trigger an alert). A variant that disappears is marked gone and keeps its history.

**Each check records**, per tracked variant
- the time
- the shown price
- the best price after vouchers, as shown on the product page
- the seller's "original" price, for display only and never used for alerts
- whether it is in stock
- whether the read succeeded, and an error if not

## Reading prices from each site

All reading happens on the product page. Nothing goes to cart or checkout.

- **Shopee.** The product page shows an "after voucher" or "best price" figure when you are logged in. Read that as the best price. If it isn't shown, the best price equals the shown price. See "What the Shopee test showed" below for where the numbers live.
- **Lazada.** Same approach as Shopee. You believe Lazada shows a similar best price on the product page, and the builder needs to confirm this.
- **Amazon Singapore.** Read the price, and subtract any "Apply $X coupon" or percentage coupon shown on the product page. Login is not required.
- **Courts.** Read the listed price. No login.

Each site has its own price reader behind one shared interface, so adding a site later means writing one new reader.

**Browser.** The Go backend uses chromedp to drive your installed Google Chrome, not headless mode, with its own dedicated profile separate from your everyday Chrome. During checks the window sits offscreen, so you don't see it. You log into Shopee and Lazada in that profile once by hand. The Settings page has a button that opens that profile visibly so you can log in again.

**Captchas.** The plan is to avoid them by looking like a real browser (real Chrome, saved profile, not headless). No extra human imitation like scrolling or mouse movement for now. You chose to add that later only if blocks show up. When a check lands on a verification or "Login Required" page, the app pauses that site, emails you once, and the Settings button opens the window so you can solve it. It never retries through a captcha, and it uses no paid solving service.

**What the Shopee test showed (2026-10-18).** A throwaway Go test drove real Chrome with chromedp.
- Logged out, Shopee redirects to `/verify/traffic/error` with a "Login Required" page. Treat that URL as "session expired."
- Logging in by hand in the visible window triggered one captcha, which you solved. After that, two offscreen checks loaded search and product pages with no wall.
- The product page itself calls `https://shopee.sg/api/v4/pdp/get_pc?item_id=...&shop_id=...`. Listening for that response in the browser is more reliable than reading the page text. Money values are integers in units of 1/100000 SGD (31500000 is $315.00).
- In that response, `data.product_price.price.single_value` is the "After Voucher" price shown on the page ($315.00). `data.item.models[]` lists variants with `model_id`, `name`, `price` ($316.00 flash deal price before voucher) and `price_before_discount` ($349.00, the seller's "was" price). `data.item.tier_variations` has variant option names. `data.item.stock_display` showed "IN STOCK".
- Open question for the builder. The after voucher figure was confirmed only for the default variant. For other variants it may require selecting the variant on the page, or using the URL's `display_model_id` parameter (seen as `?extraParams={"display_model_id":...}`), so the page loads that variant's after voucher price. If neither works, record the variant's `price` as its best price and say so in the dashboard.
- Product URLs look like `shopee.sg/<slug>-i.<shop_id>.<item_id>`.
- chromedp v0.21 changed its API to generics. The test used v0.14.2. The builder should pick deliberately.
- Lazada, Amazon, and Courts were not tested.

**Login checks.** On each Shopee or Lazada check, the reader confirms it is logged in. If not, it records the failure, marks the site's session as expired on the dashboard, and emails you once per expiry.

**Broken pages.** If a page fails to load, the price can't be found, or the layout changed, the app records a failed read rather than a wrong price, and shows the link as broken on the dashboard.

**Failure emails.** You get one email for each of these, and another only after it recovers and breaks again.
- A Shopee or Lazada session expires.
- A link starts failing to read.
- The app has had no successful check for 12 hours (builder's call on the number). This check can't live only inside the scheduler, since a dead scheduler can't report itself. Run it at startup, after wake, and on its own timer.

**Being gentle.** Checks visit one page at a time, with random pauses between pages, so traffic looks like a person browsing.

## Schedule

- Checks run every few hours. The interval is a setting. Default every 3 hours (builder's call).
- If the laptop slept through a scheduled check, run one soon after it wakes.
- You can trigger a check now for one product from its page.

## Alert rule

For each product, on each check

1. **Current best.** The lowest best price across all ticked, in stock variants of all its links, from this check.
2. **Typical price.** Take each earlier day's cheapest best price across the same variants (one value per day, so a day with many checks doesn't outweigh a day with one). The typical price is the median of those daily values over the last N days. It never includes the current check.
3. **Minimum history.** Skip alerting until the product has at least M days of history. Days count from when the first successful reading happened.
4. **Trigger.** Alert when the current best is at least X% below the typical price.
5. **No spam.** After an alert, the next alert for that product fires only when the price drops below the last alerted price. When the price climbs back above the alert threshold, this resets so a later drop alerts again (builder's call).

Defaults, all set globally in Settings and overridable per product
- X, the drop percent: 20% (builder's call, since you used 15% and 30% in examples)
- N, the window: 30 days
- M, the minimum history: 2 days, so alerts can start on day 3

Why the median. It catches a slow slide (25 steady days at $100, then a slide to $75 reads as 25% off) and it ignores a short fake spike (the seller bumps to $150 for two days then "drops" to $105, which reads as no change). A sudden sharp drop triggers it too.

Worked example. Typical price $279, today's best $229. That's 17.9% off. With X at 15%, you get an email. With X at 20%, you don't.

## Email

Sent through Resend on its free plan. You sign up, paste the API key and your email address into Settings. Resend lets you send from its shared sender address to your own account email without owning a domain. Settings has a "send test email" button.

A drop alert includes
- the product name
- the new best price, the site, the seller, and the variant
- the typical price it beat and the percent drop
- a direct link to the listing
- a note if history is thin, like "based on 3 days of history"

## Dashboard

Layout A from the mockups, built on the template's sidebar shell.

**Products page (home).** One row per product with the name, the number of links, a small trend line, the best price right now, and how far it sits from the typical price. A product with a broken link or an expired login shows a warning. An "Add product" button opens a form where you type a name and paste links, one per line, then pick variants for each.

**Product page.**
- Three numbers at the top. Best price now and which site has it, the typical price, and the alert threshold.
- The chart. One line per tracked variant, plus a dashed typical price line. Ranges for 7 days, 30 days, 90 days, and all time. Built with Recharts, which the template already includes.
- A table of links and variants with site, seller, variant, current price, last checked time, and status. Here you add or remove links, change variant ticks, edit the product's alert settings, pause tracking, and trigger a check now.

**Settings page.**
- Global defaults for X, N, M, and the check interval.
- Resend API key, your email, and the test email button.
- Shopee and Lazada login status with a button to log in again.

## How it runs

- One Go program serves the built frontend and a JSON API on one local address, like `localhost:8080`, runs the scheduler, drives Chrome, and sends email.
- Data lives in a SQLite file on your laptop.
- A macOS launch agent starts it at login and keeps it running.
- It listens only on localhost, since there is no app login.

## Left to the builder

- Database schema, API shape, and Go package layout.
- How the scheduler detects a missed check after sleep.
- The exact page selectors or embedded data each site reader uses, and how it detects login state.
- How pauses between pages are randomized.
- Error log retention and how much failure detail the dashboard shows.
- Tests. Site readers should be testable against saved copies of real pages.
