package util

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type rarityCatalogFile struct {
	PokemonCardRarities []rarityCatalogEntry `json:"pokemon_card_rarities"`
}

type rarityCatalogEntry struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

var (
	rarityCatalogOnce sync.Once
	rarityCatalogMap  map[string]string
)

var rarityFallbackAbbrev = map[string]string{
	"common":                    "C",
	"uncommon":                  "U",
	"rare":                      "R",
	"double rare":               "RR",
	"art rare":                  "AR",
	"super rare":                "SR",
	"special art rare":          "SAR",
	"ultra rare":                "UR",
	"hyper rare":                "HR",
	"secret rare":               "SR",
	"illustration rare":         "IR",
	"special illustration rare": "SIR",
}

func RarityAbbreviation(rarity string) string {
	trimmed := strings.TrimSpace(rarity)
	if trimmed == "" {
		return ""
	}
	if inline := inlineAbbreviation(trimmed); inline != "" {
		return inline
	}

	lookup := rarityCatalogLookup()
	if abbr := lookup[normalizeRarityKey(trimmed)]; abbr != "" {
		return abbr
	}
	if abbr := rarityFallbackAbbrev[normalizeRarityKey(trimmed)]; abbr != "" {
		return abbr
	}
	return ""
}

func RarityDisplay(rarity string) string {
	trimmed := strings.TrimSpace(rarity)
	if trimmed == "" {
		return ""
	}
	if inline := inlineAbbreviation(trimmed); inline != "" {
		return trimmed
	}
	if abbr := RarityAbbreviation(trimmed); abbr != "" {
		return fmt.Sprintf("%s (%s)", trimmed, abbr)
	}
	return trimmed
}

func rarityCatalogLookup() map[string]string {
	rarityCatalogOnce.Do(func() {
		rarityCatalogMap = make(map[string]string, len(rarityFallbackAbbrev))
		for name, abbr := range rarityFallbackAbbrev {
			rarityCatalogMap[name] = strings.TrimSpace(abbr)
		}

		payload, err := os.ReadFile("data/card_rarities.json")
		if err != nil {
			return
		}
		var decoded rarityCatalogFile
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return
		}
		for _, entry := range decoded.PokemonCardRarities {
			name := normalizeRarityKey(entry.Name)
			abbr := strings.TrimSpace(entry.Abbreviation)
			if name == "" || abbr == "" {
				continue
			}
			rarityCatalogMap[name] = abbr
		}
	})
	return rarityCatalogMap
}

func normalizeRarityKey(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func inlineAbbreviation(value string) string {
	start := strings.LastIndex(value, "(")
	end := strings.LastIndex(value, ")")
	if start < 0 || end <= start {
		return ""
	}
	inside := strings.TrimSpace(value[start+1 : end])
	if inside == "" {
		return ""
	}
	return strings.ToUpper(inside)
}
