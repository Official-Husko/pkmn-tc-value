package pokemonpricetracker

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

type ResolvedSet struct {
	ID          string
	Name        string
	Code        string
	EnglishName string
}

type CardEnrichment struct {
	PriceProviderSetID  string
	PriceProviderCardID string
	SetEnglishName      string
	EnglishName         string
	TotalSetNumber      string
	Rarity              string
	CardType            string
	Artist              string
	ImageURL            string
	ImageBaseURL        string
	TCGPlayerID         string
}

type Resolver struct {
	client *Client

	mu         sync.RWMutex
	setsByLang map[string][]trackerSet
}

func NewResolver(client *Client) *Resolver {
	return &Resolver{
		client:     client,
		setsByLang: make(map[string][]trackerSet),
	}
}

func (r *Resolver) ResolveCardID(ctx context.Context, set domain.Set, card domain.Card, cfg config.Config) (CardEnrichment, error) {
	remote := domain.RemoteCard{
		ID:           card.ID,
		SetID:        card.SetID,
		SetName:      card.SetName,
		SetCode:      card.SetCode,
		Language:     card.Language,
		Name:         card.Name,
		Number:       card.Number,
		ImageURL:     card.ImageURL,
		ImageBaseURL: card.ImageBaseURL,
	}
	resolvedSet, mapped, err := r.MapSetCards(ctx, set, []domain.RemoteCard{remote}, cfg)
	if err != nil {
		return CardEnrichment{}, err
	}
	enrichment := mapped[remote.ID]
	if strings.TrimSpace(enrichment.PriceProviderSetID) == "" {
		enrichment.PriceProviderSetID = resolvedSet.ID
	}
	return enrichment, nil
}

func (r *Resolver) MapSetCards(ctx context.Context, set domain.Set, cards []domain.RemoteCard, cfg config.Config) (ResolvedSet, map[string]CardEnrichment, error) {
	mapped := make(map[string]CardEnrichment, len(cards))
	if len(cards) == 0 {
		return ResolvedSet{}, mapped, nil
	}

	sets, err := r.loadSets(ctx, cfg)
	if err != nil {
		return ResolvedSet{}, nil, err
	}
	language := normalizeAPILanguage(set.Language)
	matchedSet, ok := bestSetMatch(set, sets[language])
	if !ok {
		return ResolvedSet{}, mapped, nil
	}

	resolved := ResolvedSet{
		ID:   matchedSet.TCGPlayerID.String(),
		Name: matchedSet.Name,
		Code: matchedSet.SetCode,
	}
	if resolved.ID == "" {
		resolved.ID = matchedSet.ID.String()
	}

	englishSet, foundEnglish := findEnglishSetMatch(matchedSet, sets["english"], set)
	if foundEnglish {
		resolved.EnglishName = englishSet.Name
	}

	priceCards, err := r.fetchSetCards(ctx, language, resolved.ID, resolved.Name, cfg)
	if err != nil {
		return ResolvedSet{}, nil, err
	}

	englishNamesByNumber := make(map[string]string)
	if foundEnglish {
		englishSetID := englishSet.TCGPlayerID.String()
		if englishSetID == "" {
			englishSetID = englishSet.ID.String()
		}
		englishCards, englishErr := r.fetchSetCards(ctx, "english", englishSetID, englishSet.Name, cfg)
		if englishErr == nil {
			for _, card := range englishCards {
				number := normalizeTrackerCardNumber(card.CardNumber)
				if number == "" {
					continue
				}
				if strings.TrimSpace(card.Name) != "" {
					englishNamesByNumber[number] = card.Name
				}
			}
		}
	}

	matches := matchCards(cards, priceCards)
	for remoteID, card := range matches {
		number := normalizeTrackerCardNumber(card.CardNumber)
		englishName := ""
		if language == "english" {
			englishName = card.Name
		} else if resolved.EnglishName != "" {
			englishName = englishNamesByNumber[number]
		}
		imageURL := strings.TrimSpace(card.ImageCdnURL800)
		if imageURL == "" {
			imageURL = strings.TrimSpace(card.ImageCdnURL)
		}
		mapped[remoteID] = CardEnrichment{
			PriceProviderSetID:  resolved.ID,
			PriceProviderCardID: card.TCGPlayerID.String(),
			SetEnglishName:      resolved.EnglishName,
			EnglishName:         strings.TrimSpace(englishName),
			TotalSetNumber:      strings.TrimSpace(card.TotalSetNumber),
			Rarity:              strings.TrimSpace(card.Rarity),
			CardType:            strings.TrimSpace(card.CardType),
			Artist:              strings.TrimSpace(card.Artist),
			ImageURL:            imageURL,
			ImageBaseURL:        imageURL,
			TCGPlayerID:         card.TCGPlayerID.String(),
		}
	}
	return resolved, mapped, nil
}

func (r *Resolver) loadSets(ctx context.Context, cfg config.Config) (map[string][]trackerSet, error) {
	r.mu.RLock()
	if len(r.setsByLang) > 0 {
		out := map[string][]trackerSet{
			"english":  append([]trackerSet(nil), r.setsByLang["english"]...),
			"japanese": append([]trackerSet(nil), r.setsByLang["japanese"]...),
		}
		r.mu.RUnlock()
		return out, nil
	}
	r.mu.RUnlock()

	englishSets, err := r.client.FetchSets(ctx, "english", cfg)
	if err != nil {
		return nil, err
	}
	japaneseSets, err := r.client.FetchSets(ctx, "japanese", cfg)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.setsByLang["english"] = englishSets
	r.setsByLang["japanese"] = japaneseSets
	r.mu.Unlock()

	return map[string][]trackerSet{
		"english":  englishSets,
		"japanese": japaneseSets,
	}, nil
}

func (r *Resolver) fetchSetCards(ctx context.Context, language string, setID string, setName string, cfg config.Config) ([]trackerCard, error) {
	if strings.TrimSpace(setID) != "" {
		cards, err := r.client.FetchCardsBySetID(ctx, language, setID, cfg)
		if err == nil && len(cards) > 0 {
			return cards, nil
		}
	}
	if strings.TrimSpace(setName) != "" {
		return r.client.FetchCardsBySetName(ctx, language, setName, cfg)
	}
	return nil, nil
}

func bestSetMatch(local domain.Set, candidates []trackerSet) (trackerSet, bool) {
	if len(candidates) == 0 {
		return trackerSet{}, false
	}
	wantName := util.NormalizeName(local.Name)
	wantCode := normalizeSetCode(local.SetCode)
	release := strings.TrimSpace(local.ReleaseDate)

	type scored struct {
		set   trackerSet
		score int
	}
	scoredSets := make([]scored, 0, len(candidates))
	for _, set := range candidates {
		score := 0
		name := util.NormalizeName(set.Name)
		code := normalizeSetCode(set.SetCode)
		switch {
		case name == wantName:
			score += 80
		case strings.Contains(name, wantName) || strings.Contains(wantName, name):
			score += 50
		}
		if wantCode != "" && code != "" && wantCode == code {
			score += 120
		}
		if release != "" && strings.TrimSpace(set.ReleaseDate) == release {
			score += 20
		}
		if strings.TrimSpace(local.PriceProviderSetID) != "" && strings.TrimSpace(local.PriceProviderSetID) == set.TCGPlayerID.String() {
			score += 200
		}
		if score > 0 {
			scoredSets = append(scoredSets, scored{set: set, score: score})
		}
	}
	if len(scoredSets) == 0 {
		return trackerSet{}, false
	}
	sort.SliceStable(scoredSets, func(i, j int) bool {
		if scoredSets[i].score == scoredSets[j].score {
			return scoredSets[i].set.Name < scoredSets[j].set.Name
		}
		return scoredSets[i].score > scoredSets[j].score
	})
	return scoredSets[0].set, true
}

func findEnglishSetMatch(source trackerSet, englishSets []trackerSet, local domain.Set) (trackerSet, bool) {
	sourceCode := normalizeSetCode(source.SetCode)
	sourceID := strings.TrimSpace(source.TCGPlayerID.String())
	localCode := normalizeSetCode(local.SetCode)

	for _, set := range englishSets {
		if sourceID != "" && sourceID == strings.TrimSpace(set.TCGPlayerID.String()) {
			return set, true
		}
		if sourceCode != "" && sourceCode == normalizeSetCode(set.SetCode) {
			return set, true
		}
		if localCode != "" && localCode == normalizeSetCode(set.SetCode) {
			return set, true
		}
	}
	return trackerSet{}, false
}

func normalizeSetCode(code string) string {
	value := strings.TrimSpace(strings.ToLower(code))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func matchCards(remote []domain.RemoteCard, tracker []trackerCard) map[string]trackerCard {
	out := make(map[string]trackerCard, len(remote))
	numberToCards := make(map[string][]trackerCard)
	nameToCards := make(map[string][]trackerCard)
	numberAndName := make(map[string]trackerCard)
	for _, card := range tracker {
		number := normalizeTrackerCardNumber(card.CardNumber)
		name := util.NormalizeName(card.Name)
		if number != "" {
			numberToCards[number] = append(numberToCards[number], card)
		}
		if name != "" {
			nameToCards[name] = append(nameToCards[name], card)
		}
		if number != "" && name != "" {
			numberAndName[number+"::"+name] = card
		}
	}

	for _, card := range remote {
		number := util.NormalizeCardNumber(card.Number)
		name := util.NormalizeName(card.Name)
		if exact, ok := numberAndName[number+"::"+name]; ok {
			out[card.ID] = exact
			continue
		}
		if candidates := numberToCards[number]; len(candidates) == 1 {
			out[card.ID] = candidates[0]
			continue
		}
		if candidates := nameToCards[name]; len(candidates) == 1 {
			out[card.ID] = candidates[0]
		}
	}
	return out
}

func normalizeTrackerCardNumber(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if slash := strings.Index(trimmed, "/"); slash > 0 {
		trimmed = trimmed[:slash]
	}
	return util.NormalizeCardNumber(trimmed)
}

func (r *Resolver) EnsureLinkedCard(ctx context.Context, set domain.Set, card domain.Card, cfg config.Config) (CardEnrichment, error) {
	enrichment, err := r.ResolveCardID(ctx, set, card, cfg)
	if err != nil {
		return CardEnrichment{}, fmt.Errorf("resolve tracker card id: %w", err)
	}
	return enrichment, nil
}
