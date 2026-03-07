package screens

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
)

func ShowStartupSummary(stats syncer.Stats, theme *huh.Theme) error {
	body := fmt.Sprintf(
		"New sets: %d\nUpdated sets: %d\nCards are synced per set when you open one.",
		stats.NewSets,
		stats.UpdatedSets,
	)
	return ShowMessage("Startup Sync Complete", body, theme)
}
