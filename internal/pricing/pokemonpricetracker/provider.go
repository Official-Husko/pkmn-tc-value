package pokemonpricetracker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
)

type Provider struct {
	client   *Client
	resolver *Resolver
}

func NewProvider(client *Client, resolver *Resolver) pricing.Provider {
	return &Provider{
		client:   client,
		resolver: resolver,
	}
}

func (p *Provider) Name() string {
	return "pokemonpricetracker"
}

func (p *Provider) RefreshCard(ctx context.Context, card domain.Card, set domain.Set, cfg config.Config) (domain.PriceSnapshot, error) {
	providerCardID := strings.TrimSpace(card.PriceProviderCardID)
	providerSetID := strings.TrimSpace(card.PriceProviderSetID)
	enrichment := CardEnrichment{}

	if providerCardID == "" && p.resolver != nil {
		resolved, err := p.resolver.EnsureLinkedCard(ctx, set, card, cfg)
		if err != nil {
			return domain.PriceSnapshot{}, err
		}
		enrichment = resolved
		providerCardID = strings.TrimSpace(resolved.PriceProviderCardID)
		if providerSetID == "" {
			providerSetID = strings.TrimSpace(resolved.PriceProviderSetID)
		}
	}
	if providerCardID == "" {
		providerCardID = strings.TrimSpace(card.TCGPlayerID)
	}
	if providerCardID == "" {
		return domain.PriceSnapshot{}, fmt.Errorf("missing price provider card id")
	}

	language := set.Language
	if strings.TrimSpace(language) == "" {
		language = card.Language
	}
	baseCard, err := p.client.FetchCardByID(ctx, language, providerCardID, false, cfg)
	if err != nil {
		return domain.PriceSnapshot{}, err
	}

	checkedAt := parseCheckedAt(baseCard.Prices.LastUpdated)
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	ungraded := moneyFrom(baseCard.Prices.Market)
	low := moneyFrom(firstNumber(baseCard.Prices.LowPrice, baseCard.Prices.Low))
	psa10 := findPSA10Money(baseCard.EbayData)
	sourceURL := v2BaseURL + "/cards?language=" + normalizeAPILanguage(language) + "&cardId=" + providerCardID

	if psa10 == nil {
		ebayCard, ebayErr := p.client.FetchCardByID(ctx, language, providerCardID, true, cfg)
		if ebayErr == nil {
			psa10 = findPSA10Money(ebayCard.EbayData)
		}
	}

	publicCard, detailsErr := p.client.FetchPublicDetails(ctx, language, providerCardID, cfg)
	if detailsErr == nil {
		if ungraded == nil {
			ungraded = moneyFrom(publicCard.Prices.Market)
		}
		if low == nil {
			low = moneyFrom(firstNumber(publicCard.Prices.LowPrice, publicCard.Prices.Low))
		}
		if psa10 == nil {
			psa10 = findPSA10FromVariants(publicCard.Variants)
		}
		if checked := parseCheckedAt(publicCard.Prices.LastUpdated); !checked.IsZero() {
			checkedAt = checked
		}
	}

	if psa10 == nil {
		history, historyErr := p.client.FetchInternalHistory(ctx, language, providerCardID, cfg)
		if historyErr == nil {
			psa10 = findPSA10Money(history.Data.EbayData)
		}
	}

	return domain.PriceSnapshot{
		Ungraded:             ungraded,
		Low:                  low,
		PSA10:                psa10,
		SourceURL:            sourceURL,
		CheckedAt:            checkedAt,
		PriceProviderCardID:  providerCardID,
		PriceProviderSetID:   firstNonEmpty(providerSetID, set.PriceProviderSetID),
		PriceProviderSetName: firstNonEmpty(set.PriceProviderSetName, enrichment.SetEnglishName),
		PriceProviderSetCode: firstNonEmpty(set.PriceProviderSetCode, set.PriceProviderSetID),
	}, nil
}

func moneyFrom(value *float64) *domain.Money {
	if value == nil {
		return nil
	}
	return &domain.Money{
		Amount:   *value,
		Currency: "USD",
	}
}

func parseCheckedAt(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02"} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func firstNumber(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func findPSA10FromVariants(variants map[string]variantPrice) *domain.Money {
	for name, variant := range variants {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if strings.Contains(normalized, "psa 10") && variant.MarketPrice != nil {
			return moneyFrom(variant.MarketPrice)
		}
	}
	return nil
}

func findPSA10Money(payload map[string]any) *domain.Money {
	if len(payload) == 0 {
		return nil
	}
	value, ok := findPSA10Value(payload)
	if !ok {
		return nil
	}
	return &domain.Money{Amount: value, Currency: "USD"}
}

func findPSA10Value(value any) (float64, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if normalized == "psa10" || normalized == "psa_10" || normalized == "psa 10" || normalized == "10.0" {
				if number, ok := asFloat(child); ok {
					return number, true
				}
				if nested, ok := findPSA10Value(child); ok {
					return nested, true
				}
			}
		}
		for key, child := range typed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if strings.Contains(normalized, "psa") && strings.Contains(normalized, "10") {
				if number, ok := asFloat(child); ok {
					return number, true
				}
			}
			if nested, ok := findPSA10Value(child); ok {
				return nested, true
			}
		}
	case []any:
		for _, item := range typed {
			if nested, ok := findPSA10Value(item); ok {
				return nested, true
			}
		}
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		trimmed := strings.TrimSpace(strings.ReplaceAll(typed, ",", ""))
		if trimmed == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return number, true
	case map[string]any:
		for _, key := range []string{"price", "market", "avg", "value"} {
			if nested, ok := typed[key]; ok {
				return asFloat(nested)
			}
		}
	}
	return 0, false
}
