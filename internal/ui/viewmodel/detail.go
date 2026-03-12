package viewmodel

import (
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

func DetailLines(card domain.Card) []string {
	lines := []string{
		"Name: " + card.Name,
	}
	if strings.TrimSpace(card.EnglishName) != "" && !strings.EqualFold(strings.TrimSpace(card.EnglishName), strings.TrimSpace(card.Name)) {
		lines = append(lines, "English: "+card.EnglishName)
	}
	lines = append(lines,
		"No.: "+card.Number,
		"Set: "+card.SetName,
	)
	if strings.TrimSpace(card.SetEnglishName) != "" && !strings.EqualFold(strings.TrimSpace(card.SetEnglishName), strings.TrimSpace(card.SetName)) {
		lines = append(lines, "Set EN: "+card.SetEnglishName)
	}
	if strings.TrimSpace(card.Rarity) != "" {
		lines = append(lines, "Rarity: "+card.Rarity)
	}
	lines = append(lines,
		"Market: "+util.FormatMoney(card.UngradedPrice),
		"Low: "+util.FormatMoney(card.LowPrice),
		"PSA 10: "+util.FormatMoney(card.PSA10Price),
	)
	return lines
}
