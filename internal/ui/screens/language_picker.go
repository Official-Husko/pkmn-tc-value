package screens

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func PickLanguage(sets []domain.Set, colors bool) (string, bool, error) {
	type languageCounter struct {
		display string
		count   int
	}

	byLanguage := make(map[string]languageCounter)
	for _, set := range sets {
		display := strings.TrimSpace(set.Language)
		if display == "" {
			display = "Unknown"
		}
		key := strings.ToLower(display)
		item := byLanguage[key]
		if item.display == "" {
			item.display = display
		}
		item.count++
		byLanguage[key] = item
	}

	keys := make([]string, 0, len(byLanguage))
	for key := range byLanguage {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return byLanguage[keys[i]].display < byLanguage[keys[j]].display
	})

	options := make([]SelectOption, 0, len(keys))
	for _, key := range keys {
		item := byLanguage[key]
		options = append(options, SelectOption{
			Label:       fmt.Sprintf("%s (%d sets)", item.display, item.count),
			Description: "Filter sets by this language",
			Value:       item.display,
		})
	}

	return runSelect(
		"Choose Card Language",
		"Sets will be filtered to this language only. Use / to filter.",
		options,
		colors,
		true,
		12,
	)
}
