# Dropcatch

Dropcatch tracks what products really cost on Shopee, Lazada, Amazon SG and Courts. It ignores the seller's crossed out "was" price. It keeps its own history instead, so you can tell a real sale from a fake one. When the cheapest price across your links falls well below its typical price, it emails you.

It runs on your Mac and checks prices every few hours while your laptop is awake.

## Install

You need Go, Bun (at `~/.bun/bin/bun`) and Google Chrome.

```bash
make install
```

That builds the dashboard and the `dropcatch` program, then registers it to start whenever you log in to your Mac. Open http://127.0.0.1:8080.

To run it by hand instead, use `make build` and then `./dropcatch`.

## First run

1. Open **Settings**.
2. Make a free account at [resend.com](https://resend.com) and create an API key. Paste the key, and put in the email address you signed up to Resend with. Resend's free sender only delivers to that address.
3. Click **Save**, then **Send test email**, and check your inbox.
4. Under **Logins**, click **Log in** next to Shopee. A Chrome window opens. Log in there, solve any verification, then close the window. Do the same for Lazada.
5. Go to **Products**, click **Add product**, paste your links, click **Read** on each, and tick the variants you want.

## How alerts work

For each product, every check takes the cheapest best price across every ticked, in stock variant. "Best price" means after any voucher or coupon the product page shows.

The typical price is the median of each day's cheapest price over the last 30 days. You get an email when today's price is at least 20% under that. After an alert, you only hear again when the price drops even lower, or after it climbs back and drops again.

You can change the 20%, the 30 days, and how many days of history it needs first. Do it for every product in Settings, or for one product on its page.

Dropcatch also emails you once when a Shopee or Lazada login expires, when a link stops working, and when nothing has been checked for 12 hours.

## What each site gives you

| Site | Best price | Login |
|---|---|---|
| Shopee | The "After Voucher" price, per variant | Needed |
| Lazada | The sale price. Lazada doesn't show an after voucher price on the page. | Needed |
| Amazon SG | Price minus any "Apply coupon" shown | Not needed |
| Courts | The listed price | Not needed |

## Where things live

Everything sits in `~/Library/Application Support/Dropcatch`.

- `dropcatch.db` holds your products and price history.
- `chrome-profile/` is the Chrome profile Dropcatch uses for your logins.
- `logs/dropcatch.log` is the log when it runs at login.

## Development

```bash
make test        # Go tests, then typecheck, lint and tests for the dashboard
./dropcatch      # API and built dashboard on 127.0.0.1:8080
cd web && bun run dev   # dashboard with hot reload on :5173, proxying /api to :8080
```

To build a site reader, `go run ./cmd/capture` saves what a product page loads, so you can write tests against it.
