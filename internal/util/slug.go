package util

import "strings"

func Slugify(input string) string {
	s := NormalizeName(input)
	s = strings.ReplaceAll(s, "--", "-")
	return strings.Trim(s, "-")
}
