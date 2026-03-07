package screens

import (
	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/forms"
)

func LookupCardNumber(set domain.Set, theme *huh.Theme) (string, error) {
	var value string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(forms.CardLookupTitle(set)).
				Description("Examples: 1, 001, TG01, GG35, SVP 001").
				Value(&value),
		),
	).WithTheme(theme)
	return value, form.Run()
}
