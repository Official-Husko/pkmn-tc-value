package repository

import (
	"sort"
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

type CardsRepo struct {
	store *store.Store
}

func NewCardsRepo(s *store.Store) *CardsRepo {
	return &CardsRepo{store: s}
}

func (r *CardsRepo) GetBySetAndNumber(setID, number string) (domain.Card, bool, error) {
	lookupKeys := make(map[string]struct{}, 2)
	for _, key := range lookupCardNumberVariants(number) {
		lookupKeys[key] = struct{}{}
	}
	if len(lookupKeys) == 0 {
		return domain.Card{}, false, nil
	}

	var matches []domain.Card
	err := r.store.Read(func(db *store.DB) error {
		cards := db.CardsBySet[setID]
		if cards == nil {
			return nil
		}
		set := db.Sets[setID]
		for _, card := range cards {
			card = hydrateCard(card, set)
			if !cardNumberMatchesLookup(card.Number, lookupKeys) {
				continue
			}
			if util.NormalizeCardNumber(card.Number) != "" {
				matches = append(matches, card)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Card{}, false, err
	}
	if len(matches) == 0 {
		return domain.Card{}, false, nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return cardPriority(matches[i]) < cardPriority(matches[j])
	})
	return matches[0], true, nil
}

func (r *CardsRepo) ListBySet(setID string) ([]domain.Card, error) {
	var cards []domain.Card
	err := r.store.Read(func(db *store.DB) error {
		set := db.Sets[setID]
		for _, card := range db.CardsBySet[setID] {
			cards = append(cards, hydrateCard(card, set))
		}
		return nil
	})
	return cards, err
}

func cardPriority(card domain.Card) int {
	score := 0
	name := strings.ToLower(card.Name)
	if card.Secret {
		score += 10
	}
	if strings.Contains(name, "reverse holo") {
		score += 5
	}
	return score
}

func hydrateCard(card domain.Card, set domain.Set) domain.Card {
	if strings.TrimSpace(card.SetName) == "" {
		card.SetName = set.Name
	}
	if strings.TrimSpace(card.SetEnglishName) == "" {
		card.SetEnglishName = set.EnglishName
	}
	if strings.TrimSpace(card.SetCode) == "" {
		card.SetCode = set.SetCode
	}
	if strings.TrimSpace(card.Language) == "" {
		card.Language = set.Language
	}
	if strings.TrimSpace(card.ReleaseDate) == "" {
		card.ReleaseDate = set.ReleaseDate
	}
	return card
}

func cardNumberMatchesLookup(number string, lookupKeys map[string]struct{}) bool {
	for _, key := range lookupCardNumberVariants(number) {
		if _, ok := lookupKeys[key]; ok {
			return true
		}
	}
	return false
}

func lookupCardNumberVariants(number string) []string {
	canonical := util.NormalizeCardNumber(number)
	if canonical == "" {
		return nil
	}
	trimmed := trimTrailingNumericZeros(canonical)
	if trimmed != canonical {
		return []string{canonical, trimmed}
	}
	return []string{canonical}
}

func trimTrailingNumericZeros(value string) string {
	if value == "" {
		return ""
	}
	digitStart := len(value)
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] < '0' || value[i] > '9' {
			break
		}
		digitStart = i
	}
	if digitStart == len(value) {
		return value
	}
	digits := strings.TrimLeft(value[digitStart:], "0")
	if digits == "" {
		digits = "0"
	}
	return value[:digitStart] + digits
}
