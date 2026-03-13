package viewmodel

import (
	"fmt"
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
	if card.UngradedSmartPrice != nil {
		suffix := ""
		if card.UngradedSmartMeta != nil && strings.TrimSpace(card.UngradedSmartMeta.Confidence) != "" {
			suffix = " (" + card.UngradedSmartMeta.Confidence + " confidence)"
		}
		lines = append(lines, "Ungraded Smart: "+util.FormatMoney(card.UngradedSmartPrice)+suffix)
	}
	if card.SalesVelocity != nil {
		daily := "n/a"
		if card.SalesVelocity.DailyAverage != nil {
			daily = fmt.Sprintf("%.2f/day", *card.SalesVelocity.DailyAverage)
		}
		weekly := "n/a"
		if card.SalesVelocity.WeeklyAverage != nil {
			weekly = fmt.Sprintf("%.2f/week", *card.SalesVelocity.WeeklyAverage)
		}
		lines = append(lines, fmt.Sprintf("Sales: %d total | %s | %s", card.TotalSales, daily, weekly))
	}
	if card.Population != nil && card.Population.TotalPopulation > 0 {
		gemRate := "n/a"
		if card.Population.CombinedGemRate != nil {
			gemRate = fmt.Sprintf("%.2f%%", *card.Population.CombinedGemRate)
		}
		lines = append(lines, fmt.Sprintf("Population: %d (Gems: %d, Gem Rate: %s)", card.Population.TotalPopulation, card.Population.TotalGems, gemRate))
	}
	return lines
}
