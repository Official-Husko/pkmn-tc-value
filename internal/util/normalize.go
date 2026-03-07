package util

import (
	"strings"
	"unicode"
)

func NormalizeCardNumber(input string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(strings.ToUpper(input)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func NormalizeName(input string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(strings.ToLower(input)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r) || r == '-' || r == '/' || r == ':' || r == '&':
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
