package screens

import (
	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/forms"
)

func EditSettings(cfg config.Config, theme *huh.Theme) (config.Config, error) {
	return forms.SettingsForm(cfg, theme)
}
