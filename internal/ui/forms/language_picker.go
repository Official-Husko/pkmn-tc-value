package forms

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func LanguageOptions(sets []domain.Set) []huh.Option[string] {
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

	options := make([]huh.Option[string], 0, len(keys))
	for _, key := range keys {
		item := byLanguage[key]
		label := fmt.Sprintf("%s (%d sets)", item.display, item.count)
		options = append(options, huh.NewOption(label, item.display))
	}
	return options
}
