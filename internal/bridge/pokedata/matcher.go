package pokedata

import (
	"sort"
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

func MatchRemoteCards(cards []domain.RemoteCard, priceCards []PokeCard) map[string]string {
	index := make(map[string][]PokeCard)
	for _, priceCard := range priceCards {
		key := util.NormalizeCardNumber(priceCard.Number)
		index[key] = append(index[key], priceCard)
	}

	sorted := make([]domain.RemoteCard, len(cards))
	copy(sorted, cards)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	used := make(map[string]struct{})
	matches := make(map[string]string, len(sorted))

	for _, card := range sorted {
		numberKey := util.NormalizeCardNumber(card.Number)
		nameKey := util.NormalizeName(card.Name)

		numberMatches := filterUnused(index[numberKey], used)
		nameMatches := filterByName(numberMatches, nameKey)

		switch {
		case len(nameMatches) == 1:
			matches[card.ID] = nameMatches[0].ID
			used[nameMatches[0].ID] = struct{}{}
		case len(nameMatches) > 1:
			continue
		case len(numberMatches) == 1:
			matches[card.ID] = numberMatches[0].ID
			used[numberMatches[0].ID] = struct{}{}
		}
	}

	return matches
}

func MatchLocalCard(card domain.Card, priceCards []PokeCard) string {
	numberKey := util.NormalizeCardNumber(card.Number)
	nameKey := util.NormalizeName(card.Name)

	numberMatches := make([]PokeCard, 0, 4)
	for _, priceCard := range priceCards {
		if util.NormalizeCardNumber(priceCard.Number) == numberKey {
			numberMatches = append(numberMatches, priceCard)
		}
	}
	nameMatches := filterByName(numberMatches, nameKey)

	switch {
	case len(nameMatches) == 1:
		return nameMatches[0].ID
	case len(nameMatches) > 1:
		return ""
	case len(numberMatches) == 1:
		return numberMatches[0].ID
	}

	globalNameMatches := make([]PokeCard, 0, 2)
	for _, priceCard := range priceCards {
		if util.NormalizeName(priceCard.Name) == nameKey {
			globalNameMatches = append(globalNameMatches, priceCard)
		}
	}
	if len(globalNameMatches) == 1 {
		return globalNameMatches[0].ID
	}
	return ""
}

func filterUnused(items []PokeCard, used map[string]struct{}) []PokeCard {
	if len(items) == 0 {
		return nil
	}
	out := make([]PokeCard, 0, len(items))
	for _, item := range items {
		if _, ok := used[strings.TrimSpace(item.ID)]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterByName(items []PokeCard, wantName string) []PokeCard {
	if len(items) == 0 {
		return nil
	}
	out := make([]PokeCard, 0, len(items))
	for _, item := range items {
		if util.NormalizeName(item.Name) == wantName {
			out = append(out, item)
		}
	}
	return out
}
