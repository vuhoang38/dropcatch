package notify

import (
	"fmt"
	"math"
	"strings"
	"time"

	"dropcatch/internal/money"
)

// SiteName is how a site reads in emails and on the dashboard.
func SiteName(site string) string {
	switch site {
	case "shopee":
		return "Shopee"
	case "lazada":
		return "Lazada"
	case "amazon":
		return "Amazon SG"
	case "courts":
		return "Courts"
	}
	return site
}

type DropAlert struct {
	Product, Site, Seller, Variant, URL string
	NewCents, TypicalCents              int64
	DropPct                             float64
	HistoryDays, WindowDays             int
}

func DropEmail(a DropAlert) (subject, text string) {
	subject = fmt.Sprintf("%s dropped to %s (%d%% under typical)", a.Product, money.Format(a.NewCents), int(math.Round(a.DropPct)))

	var where []string
	for _, s := range []string{SiteName(a.Site), a.Seller, a.Variant} {
		if s != "" {
			where = append(where, s)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n\n", a.Product, strings.Join(where, " · "))
	fmt.Fprintf(&b, "Now: %s\n", money.Format(a.NewCents))
	fmt.Fprintf(&b, "Typical price over the last %d days: %s\n", a.WindowDays, money.Format(a.TypicalCents))
	fmt.Fprintf(&b, "That's %.1f%% under typical.\n", a.DropPct)
	if a.HistoryDays < a.WindowDays {
		fmt.Fprintf(&b, "Based on %d days of history.\n", a.HistoryDays)
	}
	fmt.Fprintf(&b, "\n%s\n", a.URL)
	return subject, b.String()
}

func SessionEmail(site string, blocked bool) (subject, text string) {
	name := SiteName(site)
	if blocked {
		return fmt.Sprintf("Dropcatch: %s wants a verification check", name),
			fmt.Sprintf("%s showed a verification page, so Dropcatch paused checking %s.\n\nOpen Dropcatch, go to Settings, and click Log in next to %s. Solve the check in the window that opens, then close it.\n", name, name, name)
	}
	return fmt.Sprintf("Dropcatch: %s login expired", name),
		fmt.Sprintf("Your %s login expired, so Dropcatch paused checking %s and can't see your vouchers.\n\nOpen Dropcatch, go to Settings, and click Log in next to %s. Log in in the window that opens, then close it.\n", name, name, name)
}

func BrokenLinkEmail(product, url, errMsg string) (subject, text string) {
	return fmt.Sprintf("Dropcatch: a %s link stopped working", product),
		fmt.Sprintf("Dropcatch couldn't read a price for %s.\n\n%s\n\nError: %s\n\nIf the page moved or the listing ended, remove or replace the link in Dropcatch.\n", product, url, errMsg)
}

func StaleEmail(lastSuccess time.Time) (subject, text string) {
	when := "ever"
	if !lastSuccess.IsZero() {
		when = "since " + lastSuccess.Format("Mon 2 Jan 15:04")
	}
	return "Dropcatch hasn't checked any prices in 12 hours",
		fmt.Sprintf("Dropcatch hasn't read a single price %s, so it can't catch drops.\n\nOpen Dropcatch and look for broken links or expired logins. The log is in ~/Library/Application Support/Dropcatch/logs.\n", when)
}

func TestEmail() (subject, text string) {
	return "Dropcatch test email", "If you're reading this, Dropcatch can email you about price drops.\n"
}
