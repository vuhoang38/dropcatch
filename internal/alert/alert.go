// Package alert decides when a product's price has dropped enough to email.
package alert

import "time"

// SGT is Singapore time. Days are always Singapore calendar days.
var SGT = time.FixedZone("SGT", 8*3600)

// DayPrice is the cheapest best price seen on one Singapore day.
type DayPrice struct {
	Day   string // "2006-01-02" in SGT
	Cents int64
}
