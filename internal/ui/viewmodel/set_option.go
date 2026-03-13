package viewmodel

import (
	"fmt"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func SetLabel(set domain.Set) string {
	if set.Series == "" {
		if set.SetCode != "" {
			return fmt.Sprintf("[%s] %s · %d cards", set.SetCode, set.Name, set.Total)
		}
		return fmt.Sprintf("%s · %d cards", set.Name, set.Total)
	}
	if set.SetCode != "" {
		return fmt.Sprintf("[%s] %s - %s · %d cards", set.SetCode, set.Series, set.Name, set.Total)
	}
	return fmt.Sprintf("%s - %s · %d cards", set.Series, set.Name, set.Total)
}
