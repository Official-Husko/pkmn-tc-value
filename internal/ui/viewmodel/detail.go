package viewmodel

import (
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

func DetailLines(card domain.Card) []string {
	lines := []string{
		"Name: " + card.Name,
		"No.: " + card.Number,
		"Set: " + card.SetName,
	}
	if strings.TrimSpace(card.Rarity) != "" {
		lines = append(lines, "Rarity: "+card.Rarity)
	}
	lines = append(lines,
		"Ungraded: "+util.FormatMoney(card.UngradedPrice),
		"PSA 10: "+util.FormatMoney(card.PSA10Price),
	)
	return lines
}
