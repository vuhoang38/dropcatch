// Package money handles SGD amounts as integer cents.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var errBadPrice = errors.New("not a price")

// ParseSGD turns a displayed price like "$315.00", "S$1,299" or "SGD 27.50"
// into cents. A zero price is an error, since no listing really sells for $0.
func ParseSGD(s string) (int64, error) {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == ',' || r == '$' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "SGD"), "S")

	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || (hasFrac && frac == "") {
		return 0, fmt.Errorf("%w: %q", errBadPrice, s)
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errBadPrice, s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errBadPrice, s)
	}
	cents := w*100 + f
	if cents <= 0 {
		return 0, fmt.Errorf("%w: %q", errBadPrice, s)
	}
	return cents, nil
}

// Format renders cents as "$1,299.00".
func Format(cents int64) string {
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("$%s.%02d", b.String(), cents%100)
}
