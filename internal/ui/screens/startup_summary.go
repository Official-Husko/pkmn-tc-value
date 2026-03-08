package screens

import (
	"fmt"

	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
)

func ShowStartupSummary(stats syncer.Stats, colors bool) error {
	body := fmt.Sprintf(
		"New sets: %d\nUpdated sets: %d\nCards are synced per set when you open one.",
		stats.NewSets,
		stats.UpdatedSets,
	)
	return ShowMessage("Startup Sync Complete", body, colors)
}
