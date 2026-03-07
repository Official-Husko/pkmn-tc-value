package screens

import (
	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/forms"
)

func PickLanguage(sets []domain.Set, theme *huh.Theme) (string, error) {
	var language string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose card language").
				Description("Sets will be filtered to this language only.").
				Filtering(true).
				Height(10).
				Options(forms.LanguageOptions(sets)...).
				Value(&language),
		),
	).WithTheme(theme)
	return language, form.Run()
}
