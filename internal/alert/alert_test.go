package alert

import (
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 18, 10, 0, 0, 0, SGT)

func day(ago int) string { return now.AddDate(0, 0, -ago).Format("2006-01-02") }

// flat returns days ago from..to (inclusive, from > to) all at cents.
func flat(from, to int, cents int64) []DayPrice {
	var out []DayPrice
	for ago := from; ago >= to; ago-- {
		out = append(out, DayPrice{Day: day(ago), Cents: cents})
	}
	return out
}

func ptr(n int64) *int64 { return &n }

func input(current int64, daily []DayPrice) Input {
	return Input{
		Now:          now,
		CurrentCents: ptr(current),
		Daily:        daily,
		FirstDay:     day(60),
		Params:       Params{DropPct: 20, WindowDays: 30, MinHistoryDays: 2},
	}
}

func TestMedian(t *testing.T) {
	cases := []struct {
		in   []int64
		want int64
	}{{[]int64{3, 1, 2}, 2}, {[]int64{1, 2, 3, 4}, 3}, {[]int64{279}, 279}, {[]int64{100, 201}, 151}}
	for _, c := range cases {
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v) = %d; want %d", c.in, got, c.want)
		}
	}
}

func TestSpecWorkedExample(t *testing.T) {
	in := input(22900, flat(30, 1, 27900))
	in.Params.DropPct = 15
	got := Evaluate(in)
	if !got.Alert || got.TypicalCents != 27900 || math.Abs(got.DropPct-17.92) > 0.01 {
		t.Fatalf("15%%: %+v", got)
	}
	in.Params.DropPct = 20
	if got := Evaluate(in); got.Alert {
		t.Fatalf("20%%: %+v", got)
	}
}

func TestSlowSlide(t *testing.T) {
	daily := append(flat(29, 5, 10000), DayPrice{day(4), 9500}, DayPrice{day(3), 9000}, DayPrice{day(2), 8500}, DayPrice{day(1), 8000})
	in := input(7500, daily)
	in.Params.DropPct = 25
	got := Evaluate(in)
	if !got.Alert || got.TypicalCents != 10000 {
		t.Fatalf("%+v", got)
	}
}

func TestFakeSpikeIgnored(t *testing.T) {
	daily := append(flat(30, 3, 10000), flat(2, 1, 15000)...)
	got := Evaluate(input(10500, daily))
	if got.Alert || got.TypicalCents != 10000 {
		t.Fatalf("%+v", got)
	}
}

func TestTodayExcluded(t *testing.T) {
	daily := append(flat(3, 1, 27900), DayPrice{day(0), 1000})
	got := Evaluate(input(27900, daily))
	if got.TypicalCents != 27900 {
		t.Fatalf("%+v", got)
	}
}

func TestMinHistory(t *testing.T) {
	in := input(10000, flat(1, 1, 27900))
	in.FirstDay = day(1)
	if got := Evaluate(in); got.Alert || got.HistoryDays != 1 {
		t.Fatalf("1 day: %+v", got)
	}
	in.FirstDay = day(2)
	in.Daily = flat(2, 1, 27900)
	if got := Evaluate(in); !got.Alert || got.HistoryDays != 2 {
		t.Fatalf("2 days: %+v", got)
	}
}

func TestNoSpam(t *testing.T) {
	in := input(20000, flat(30, 1, 27900))
	in.LastAlertedCents = ptr(20000)
	if got := Evaluate(in); got.Alert || *got.LastAlertedCents != 20000 {
		t.Fatalf("same price: %+v", got)
	}
	in.CurrentCents = ptr(19900)
	got := Evaluate(in)
	if !got.Alert || *got.LastAlertedCents != 19900 {
		t.Fatalf("lower price: %+v", got)
	}
}

func TestReset(t *testing.T) {
	in := input(27900, flat(30, 1, 27900))
	in.LastAlertedCents = ptr(20000)
	if got := Evaluate(in); got.Alert || got.LastAlertedCents != nil {
		t.Fatalf("%+v", got)
	}
}

func TestNoCurrentKeepsState(t *testing.T) {
	in := input(0, flat(30, 1, 27900))
	in.CurrentCents = nil
	in.LastAlertedCents = ptr(20000)
	if got := Evaluate(in); got.Alert || got.LastAlertedCents == nil || *got.LastAlertedCents != 20000 {
		t.Fatalf("%+v", got)
	}
}

func TestNoTypicalNoAlert(t *testing.T) {
	if got := Evaluate(input(100, nil)); got.Alert || got.HasTypical {
		t.Fatalf("%+v", got)
	}
}

func TestWindowEdge(t *testing.T) {
	daily := append([]DayPrice{{day(31), 1}}, flat(30, 1, 27900)...)
	if got := Evaluate(input(27900, daily)); got.TypicalCents != 27900 {
		t.Fatalf("%+v", got)
	}
}
