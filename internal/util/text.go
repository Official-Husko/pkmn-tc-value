package util

import (
	"html"
	"strconv"
	"strings"
)

// DecodeEscapedText normalizes values that can contain escaped unicode sequences
// (for example "\u0026") and HTML entities.
func DecodeEscapedText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	quoted := `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	if decoded, err := strconv.Unquote(quoted); err == nil {
		value = decoded
	}

	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}
