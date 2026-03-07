package forms

import (
	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/viewmodel"
)

func SetOptions(sets []domain.Set) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(sets))
	for _, set := range sets {
		options = append(options, huh.NewOption(viewmodel.SetLabel(set), set.ID))
	}
	return options
}
