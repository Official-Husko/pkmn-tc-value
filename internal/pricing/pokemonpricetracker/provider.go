package pokemonpricetracker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
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
	return "pokewallet"
}

func (p *Provider) RefreshCard(ctx context.Context, card domain.Card, set domain.Set, cfg config.Config) (domain.PriceSnapshot, error) {
	providerCardID := strings.TrimSpace(card.PriceProviderCardID)
	tcgPlayerID := strings.TrimSpace(card.TCGPlayerID)
	if tcgPlayerID == "" && isDigits(providerCardID) {
		// Legacy rows may still have tcgplayer IDs stored as provider card IDs.
		tcgPlayerID = providerCardID
		providerCardID = ""
	}
	providerSetID := firstNonEmpty(card.PriceProviderSetID, set.PriceProviderSetID)
	enrichment := CardEnrichment{}
	var resolveErr error

	if (providerCardID == "" || tcgPlayerID == "") && p.resolver != nil {
		resolved, err := p.resolver.EnsureLinkedCard(ctx, set, card, cfg)
		if err != nil {
			resolveErr = err
		} else {
			enrichment = resolved
			providerCardID = firstNonEmpty(providerCardID, resolved.PriceProviderCardID)
			tcgPlayerID = firstNonEmpty(tcgPlayerID, resolved.TCGPlayerID)
			if providerSetID == "" {
				providerSetID = strings.TrimSpace(resolved.PriceProviderSetID)
			}
		}
	}
	if providerCardID == "" && tcgPlayerID == "" {
		if resolveErr != nil {
			return domain.PriceSnapshot{}, fmt.Errorf("missing pokewallet card id/tcgplayer id (resolver failed: %w)", resolveErr)
		}
		return domain.PriceSnapshot{}, fmt.Errorf("missing pokewallet card id/tcgplayer id")
	}

	language := set.Language
	if strings.TrimSpace(language) == "" {
		language = card.Language
	}
	sourceURL := ""
	if lookupID := walletLookupCardID(providerCardID, tcgPlayerID); lookupID != "" {
		sourceURL = fmt.Sprintf("%s/cards/%s", pokewalletBaseURL, lookupID)
	}
	if sourceURL == "" && tcgPlayerID != "" {
		sourceURL = fmt.Sprintf("%s/cards/%s/details?days=30&language=%s", pokepricePublicBaseURL, tcgPlayerID, normalizeAPILanguage(language))
	}

	var walletCard trackerCard
	var walletErr error
	walletCardID := walletLookupCardID(providerCardID, tcgPlayerID)
	if walletCardID != "" {
		walletCard, walletErr = p.client.FetchCardByID(ctx, language, walletCardID, false, cfg)
		tcgPlayerID = firstNonEmpty(tcgPlayerID, walletCard.TCGPlayerID.String())
	}

	var publicCard trackerCard
	var publicErr error
	if tcgPlayerID != "" {
		publicCard, publicErr = p.client.FetchPublicDetails(ctx, language, tcgPlayerID, cfg)
	}
	var history historyEnvelope
	var historyErr error
	if tcgPlayerID != "" {
		history, historyErr = p.client.FetchInternalHistory(ctx, language, tcgPlayerID, cfg)
	}

	if walletErr != nil && publicErr != nil {
		return domain.PriceSnapshot{}, fmt.Errorf(
			"refresh failed for card=%s tcg=%s: pokewallet: %v; pokeprice public: %v",
			providerCardID,
			tcgPlayerID,
			walletErr,
			publicErr,
		)
	}
	if walletErr != nil && tcgPlayerID == "" && publicErr == nil {
		tcgPlayerID = publicCard.TCGPlayerID.String()
	}

	checkedAt := parseCheckedAt(publicCard.Prices.LastUpdated)
	if checkedAt.IsZero() {
		checkedAt = parseCheckedAt(walletCard.Prices.LastUpdated)
	}
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	ungraded := moneyFrom(firstNumber(publicCard.Prices.Market, walletCard.Prices.Market))
	low := moneyFrom(firstNumber(publicCard.Prices.LowPrice, publicCard.Prices.Low, walletCard.Prices.LowPrice, walletCard.Prices.Low))
	psa10 := findPSA10FromVariants(publicCard.Variants)

	if ungraded == nil {
		ungraded = moneyFrom(firstNumber(publicCard.Variants["Normal"].MarketPrice, walletCard.Variants["Normal"].MarketPrice))
	}
	if low == nil {
		low = moneyFrom(firstNumber(publicCard.Variants["Normal"].LowPrice, walletCard.Variants["Normal"].LowPrice))
	}

	ebay := pickEbayData(publicCard.Ebay, nil)
	if ebay == nil && history.Data.Ebay != nil {
		ebay = history.Data.Ebay
	}
	if ebay == nil && len(history.Data.EbayData) > 0 {
		decoded, decodeErr := decodeEbayData(history.Data.EbayData)
		if decodeErr == nil {
			ebay = decoded
		}
	}
	if psa10 == nil {
		psa10 = findPSA10FromEbay(ebay)
	}
	if psa10 == nil && len(history.Data.EbayData) > 0 {
		psa10 = findPSA10Money(history.Data.EbayData)
	}

	setName := firstNonEmpty(publicCard.SetName, walletCard.SetName, set.Name, card.SetName)
	cardName := firstNonEmpty(publicCard.Name, walletCard.Name, card.Name)
	cardNumber := util.CardLocalNumber(firstNonEmpty(publicCard.CardNumber, walletCard.CardNumber, card.Number))
	totalSetNumber := firstNonEmpty(publicCard.TotalSetNumber, walletCard.TotalSetNumber, card.TotalSetNumber)
	rarity := firstNonEmpty(publicCard.Rarity, walletCard.Rarity, card.Rarity)
	cardType := firstNonEmpty(publicCard.CardType, walletCard.CardType, card.CardType)
	artist := firstNonEmpty(publicCard.Artist, walletCard.Artist, card.Artist)
	imageURL := firstNonEmpty(
		publicCard.ImageCdnURL800,
		publicCard.ImageCdnURL,
		publicCard.ImageURL,
		walletCard.ImageCdnURL800,
		walletCard.ImageCdnURL,
		walletCard.ImageURL,
		card.ImageURL,
	)
	imageBaseURL := firstNonEmpty(
		publicCard.ImageCdnURL800,
		publicCard.ImageCdnURL,
		walletCard.ImageCdnURL800,
		walletCard.ImageCdnURL,
		card.ImageBaseURL,
	)

	if strings.TrimSpace(sourceURL) == "" && tcgPlayerID != "" {
		sourceURL = fmt.Sprintf("%s/cards/%s/details?days=30&language=%s", pokepricePublicBaseURL, tcgPlayerID, normalizeAPILanguage(language))
	}

	gradeWorth := gradeWorthFromEbay(ebay)
	ungradedSmartPrice, ungradedSmartMeta := ungradedSmartFromWorth(gradeWorth)
	salesVelocity := salesVelocityFromEbay(ebay)
	totalSales := 0
	var totalSalesValue *domain.Money
	recentSales := recentSalesFromEbay(ebay)
	if ebay != nil {
		totalSales = ebay.TotalSales
		totalSalesValue = moneyFrom(ebay.TotalValue)
	}
	population := populationFromHistory(history.Data.Population)
	if population == nil {
		population = populationFromHistory(publicCard.Population)
	}

	_ = historyErr // keep refresh resilient even if this enrichment call fails.

	return domain.PriceSnapshot{
		Ungraded:             ungraded,
		Low:                  low,
		PSA10:                psa10,
		GradeWorth:           gradeWorth,
		UngradedSmartPrice:   ungradedSmartPrice,
		UngradedSmartMeta:    ungradedSmartMeta,
		SalesVelocity:        salesVelocity,
		TotalSales:           totalSales,
		TotalSalesValue:      totalSalesValue,
		RecentSales:          recentSales,
		Population:           population,
		SourceURL:            sourceURL,
		CheckedAt:            checkedAt,
		TCGPlayerID:          tcgPlayerID,
		SetName:              setName,
		CardName:             cardName,
		CardNumber:           cardNumber,
		TotalSetNumber:       totalSetNumber,
		Rarity:               rarity,
		CardType:             cardType,
		Artist:               artist,
		ImageURL:             imageURL,
		ImageBaseURL:         imageBaseURL,
		PriceProviderCardID:  normalizeStoredProviderCardID(firstNonEmpty(providerCardID, card.PriceProviderCardID)),
		PriceProviderSetID:   firstNonEmpty(providerSetID, set.PriceProviderSetID),
		PriceProviderSetName: firstNonEmpty(set.PriceProviderSetName, enrichment.SetEnglishName, setName),
		PriceProviderSetCode: firstNonEmpty(set.PriceProviderSetCode, set.SetCode),
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
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
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

func walletLookupCardID(storedProviderCardID string, tcgPlayerID string) string {
	trimmed := strings.TrimSpace(storedProviderCardID)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "pk_") {
		return trimmed
	}
	// Stored IDs strip "pk_" to keep DB clean. Add it back for TCGPlayer-linked cards.
	if strings.TrimSpace(tcgPlayerID) != "" && !isDigits(trimmed) {
		return "pk_" + trimmed
	}
	return trimmed
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

func pickEbayData(values ...*trackerEbayData) *trackerEbayData {
	for _, value := range values {
		if value == nil {
			continue
		}
		if len(value.SalesByGrade) > 0 || len(value.SoldListings) > 0 || value.TotalSales > 0 || value.TotalValue != nil {
			return value
		}
	}
	return nil
}

func decodeEbayData(payload map[string]any) (*trackerEbayData, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out trackerEbayData
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func findPSA10FromEbay(ebay *trackerEbayData) *domain.Money {
	if ebay == nil || len(ebay.SalesByGrade) == 0 {
		return nil
	}
	for gradeKey, grade := range ebay.SalesByGrade {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(gradeKey), " ", ""))
		if normalized == "psa10" || normalized == "10.0" {
			if grade.SmartMarketPrice != nil && grade.SmartMarketPrice.Price != nil {
				return moneyFrom(grade.SmartMarketPrice.Price)
			}
			return moneyFrom(firstNumber(grade.MarketPrice7Day, grade.MarketPriceMedian7Day, grade.MedianPrice, grade.AveragePrice, grade.MaxPrice, grade.MinPrice))
		}
	}
	return nil
}

func gradeWorthFromEbay(ebay *trackerEbayData) map[string]domain.GradeWorth {
	if ebay == nil || len(ebay.SalesByGrade) == 0 {
		return nil
	}
	out := make(map[string]domain.GradeWorth, len(ebay.SalesByGrade))
	for grade, data := range ebay.SalesByGrade {
		normalized := normalizeGradeKey(grade)
		if normalized == "" {
			normalized = strings.ToLower(strings.TrimSpace(grade))
		}
		var smart *domain.SmartPrice
		if data.SmartMarketPrice != nil {
			smart = &domain.SmartPrice{
				Price:      moneyFrom(data.SmartMarketPrice.Price),
				Confidence: strings.TrimSpace(data.SmartMarketPrice.Confidence),
				Method:     strings.TrimSpace(data.SmartMarketPrice.Method),
				DaysUsed:   data.SmartMarketPrice.DaysUsed,
			}
			if smart.Price == nil && smart.Confidence == "" && smart.Method == "" && smart.DaysUsed == 0 {
				smart = nil
			}
		}

		out[normalized] = domain.GradeWorth{
			Count:                 data.Count,
			TotalValue:            moneyFrom(data.TotalValue),
			AveragePrice:          moneyFrom(data.AveragePrice),
			MedianPrice:           moneyFrom(data.MedianPrice),
			MinPrice:              moneyFrom(data.MinPrice),
			MaxPrice:              moneyFrom(data.MaxPrice),
			MarketPrice7Day:       moneyFrom(data.MarketPrice7Day),
			MarketPriceMedian7Day: moneyFrom(data.MarketPriceMedian7Day),
			DailyVolume7Day:       data.DailyVolume7Day,
			MarketTrend:           strings.TrimSpace(data.MarketTrend),
			SmartMarketPrice:      smart,
		}
	}
	return out
}

func ungradedSmartFromWorth(worth map[string]domain.GradeWorth) (*domain.Money, *domain.SmartPrice) {
	if len(worth) == 0 {
		return nil, nil
	}
	for _, key := range []string{"ungraded", "raw", "normal"} {
		item, ok := worth[key]
		if !ok {
			continue
		}
		if item.SmartMarketPrice != nil {
			return item.SmartMarketPrice.Price, item.SmartMarketPrice
		}
		return firstMoney(item.MarketPrice7Day, item.MarketPriceMedian7Day, item.MedianPrice, item.AveragePrice), nil
	}
	return nil, nil
}

func salesVelocityFromEbay(ebay *trackerEbayData) *domain.SalesVelocity {
	if ebay == nil {
		return nil
	}
	velocity := &domain.SalesVelocity{
		DailyAverage:  ebay.SalesVelocity.DailyAverage,
		WeeklyAverage: ebay.SalesVelocity.WeeklyAverage,
		MonthlyTotal:  ebay.SalesVelocity.MonthlyTotal,
	}
	if velocity.DailyAverage == nil && velocity.WeeklyAverage == nil && velocity.MonthlyTotal == 0 {
		return nil
	}
	return velocity
}

func recentSalesFromEbay(ebay *trackerEbayData) []domain.SoldListing {
	if ebay == nil || len(ebay.SoldListings) == 0 {
		return nil
	}
	const maxStoredListings = 120
	out := make([]domain.SoldListing, 0, 24)
	for gradeKey, listings := range ebay.SoldListings {
		gradeLabel := normalizeGradeLabel(gradeKey)
		for _, listing := range listings {
			if listing.Price == nil {
				continue
			}
			soldAt := parseCheckedAt(listing.SoldDate)
			var soldAtPtr *time.Time
			if !soldAt.IsZero() {
				soldAtPtr = &soldAt
			}
			grade := gradeLabel
			if strings.TrimSpace(grade) == "" {
				grade = normalizeGradeLabel(listing.GradingCompany)
			}
			out = append(out, domain.SoldListing{
				Grade:  grade,
				Title:  strings.TrimSpace(listing.Title),
				Price:  moneyFrom(listing.Price),
				SoldAt: soldAtPtr,
				URL:    strings.TrimSpace(listing.URL),
			})
			if len(out) >= maxStoredListings {
				return out
			}
		}
	}
	return out
}

func normalizeGradeLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToUpper(strings.ReplaceAll(trimmed, " ", ""))
	switch normalized {
	case "PSA10":
		return "PSA 10"
	case "PSA9":
		return "PSA 9"
	case "PSA8":
		return "PSA 8"
	case "PSA7":
		return "PSA 7"
	case "CGC10":
		return "CGC 10"
	case "BGS10":
		return "BGS 10"
	case "TAG10":
		return "TAG 10"
	case "UNGRADED":
		return "Ungraded"
	default:
		return trimmed
	}
}

func populationFromHistory(pop *trackerPopulationData) *domain.PopulationSummary {
	if pop == nil {
		return nil
	}
	out := &domain.PopulationSummary{
		TotalPopulation: pop.TotalPopulation,
		TotalGems:       pop.TotalGems,
		CombinedGemRate: pop.CombinedGemRate,
		MatchConfidence: strings.TrimSpace(pop.MatchConfidence),
		MatchScore:      pop.MatchScore,
	}
	if out.TotalPopulation == 0 && out.TotalGems == 0 && out.CombinedGemRate == nil && out.MatchConfidence == "" && out.MatchScore == nil {
		return nil
	}
	return out
}

func normalizeGradeKey(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "_", "")
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	return trimmed
}

func firstMoney(values ...*domain.Money) *domain.Money {
	for _, value := range values {
		if value != nil {
			return value
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

func isDigits(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
