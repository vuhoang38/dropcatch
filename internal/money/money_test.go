package money

import "testing"

func TestParseSGD(t *testing.T) {
	ok := map[string]int64{
		"$315.00":    31500,
		"S$1,299":    129900,
		"SGD 27.50":  2750,
		"$\n207.00":  20700,
		"$ 2,098.00": 209800,
		"$27.5":      2750,
	}
	for in, want := range ok {
		got, err := ParseSGD(in)
		if err != nil || got != want {
			t.Errorf("ParseSGD(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"$0.00", "", "Free", "$1.234"} {
		if got, err := ParseSGD(in); err == nil {
			t.Errorf("ParseSGD(%q) = %d; want error", in, got)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[int64]string{129900: "$1,299.00", 2750: "$27.50", 5: "$0.05", 123456789: "$1,234,567.89"}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q; want %q", in, got, want)
		}
	}
}
