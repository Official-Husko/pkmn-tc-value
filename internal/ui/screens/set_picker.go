package screens

import (
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/viewmodel"
)

func PickSet(sets []domain.Set, colors bool) (string, bool, error) {
	options := make([]SelectOption, 0, len(sets))
	for _, set := range sets {
		options = append(options, SelectOption{
			Label:       viewmodel.SetLabel(set),
			Description: set.ReleaseDate,
			Value:       set.ID,
		})
	}
	return runSelect(
		"Choose a Set",
		"Type / to filter the set list.",
		options,
		colors,
		true,
		16,
	)
}
