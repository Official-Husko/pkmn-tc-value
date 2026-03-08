package screens

import (
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func LookupCardNumber(set domain.Set, colors bool) (string, bool, error) {
	return runTextInput(
		"Card number for "+set.Name,
		"Examples: 1, 001, TG01, GG35, SVP 001",
		"",
		colors,
	)
}
