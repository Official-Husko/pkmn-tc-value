package screens

import (
	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/forms"
)

func PickSet(sets []domain.Set, theme *huh.Theme) (string, error) {
	var setID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose a set").
				Description("Type / to filter the list.").
				Filtering(true).
				Height(15).
				Options(forms.SetOptions(sets)...).
				Value(&setID),
		),
	).WithTheme(theme)
	return setID, form.Run()
}
