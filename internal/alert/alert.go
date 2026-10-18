// Package alert decides when a product's price has dropped enough to email.
package alert

import (
	"slices"
	"time"
)

// SGT is Singapore time. Days are always Singapore calendar days.
var SGT = time.FixedZone("SGT", 8*3600)

// DayPrice is the cheapest best price seen on one Singapore day.
type DayPrice struct {
	Day   string // "2006-01-02" in SGT
	Cents int64
}

type Params struct {
	DropPct        float64
	WindowDays     int
	MinHistoryDays int
}

type Input struct {
	Now          time.Time
	CurrentCents *int64     // nil when nothing readable this check
	Daily        []DayPrice // prior days; today and days outside the window are ignored
	FirstDay     string     // SGT day of the first successful reading, "" if none
	Params       Params
	// Price of the last drop email, nil if none since the last reset.
	LastAlertedCents *int64
}

type Result struct {
	Alert        bool
	HasTypical   bool
	TypicalCents int64
	DropPct      float64 // percent below typical, negative when above
	HistoryDays  int
	// The value to store after this check.
	LastAlertedCents *int64
}

// Evaluate applies the rule from the spec: alert when the current best price is
// at least DropPct below the median daily cheapest price of the last
// WindowDays, once there's MinHistoryDays of history, and only again when the
// price beats the last alerted price.
func Evaluate(in Input) Result {
	today := dayStart(in.Now)
	windowStart := today.AddDate(0, 0, -in.Params.WindowDays).Format(dayLayout)
	todayKey := today.Format(dayLayout)

	var window []int64
	for _, d := range in.Daily {
		if d.Day >= windowStart && d.Day < todayKey {
			window = append(window, d.Cents)
		}
	}

	res := Result{LastAlertedCents: in.LastAlertedCents}
	if first, err := time.ParseInLocation(dayLayout, in.FirstDay, SGT); err == nil {
		res.HistoryDays = int(today.Sub(first).Hours()/24 + 0.5)
	}
	if len(window) == 0 {
		return res
	}
	res.HasTypical = true
	res.TypicalCents = Median(window)
	if in.CurrentCents == nil {
		return res
	}
	current := *in.CurrentCents
	res.DropPct = float64(res.TypicalCents-current) / float64(res.TypicalCents) * 100

	switch {
	case res.DropPct < in.Params.DropPct:
		res.LastAlertedCents = nil
	case res.HistoryDays < in.Params.MinHistoryDays:
	case in.LastAlertedCents != nil && current >= *in.LastAlertedCents:
	default:
		res.Alert = true
		res.LastAlertedCents = &current
	}
	return res
}

const dayLayout = "2006-01-02"

func dayStart(t time.Time) time.Time {
	t = t.In(SGT)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, SGT)
}

// DayStart is midnight Singapore time on t's Singapore day.
func DayStart(t time.Time) time.Time { return dayStart(t) }

// Median of xs. For an even count, the mean of the middle two rounded half up.
func Median(xs []int64) int64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	sum := s[n/2-1] + s[n/2]
	return (sum + 1) / 2
}
