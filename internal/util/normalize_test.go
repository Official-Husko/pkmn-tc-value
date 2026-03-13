package util

import "testing"

func TestNormalizeCardNumber(t *testing.T) {
	cases := map[string]string{
		"001":     "001",
		"SVP 001": "SVP001",
		"tg01":    "TG01",
		"GG-35":   "GG35",
		"073/190": "073",
	}
	for input, want := range cases {
		if got := NormalizeCardNumber(input); got != want {
			t.Fatalf("NormalizeCardNumber(%q) = %q, want %q", input, got, want)
		}
	}
}
