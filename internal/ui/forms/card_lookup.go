package forms

import "github.com/Official-Husko/pkmn-tc-value/internal/domain"

func CardLookupTitle(set domain.Set) string {
	return "Card number for " + set.Name
}
