package pokemonpricetracker

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestRefreshCardUsesPokewalletCardAndPublicDetails(t *testing.T) {
	client := &Client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/cards/pk_test_card") && strings.Contains(req.Host, "api.pokewallet.io") {
					body := `{"id":"pk_test_card","card_info":{"name":"Defiance Band","clean_name":"Defiance Band","set_name":"SV4a: Shiny Treasure ex","set_code":"SV4A","set_id":"23601","card_number":"168/190","rarity":"None","card_type":"Trainer"},"tcgplayer":{"url":"https://www.tcgplayer.com/product/567569","prices":[{"sub_type_name":"Normal","market_price":0.05,"low_price":0.01,"updated_at":"2026-03-11T10:26:30.551818"}]}}`
					return testResponse(http.StatusOK, body), nil
				}
				if strings.Contains(req.URL.Path, "/api/cards/567569/details") && strings.Contains(req.Host, "www.pokemonpricetracker.com") {
					body := `{"success":true,"data":{"tcgPlayerId":"567569","setName":"SV4a: Shiny Treasure ex","name":"Defiance Band","cardNumber":"168/190","rarity":"None","cardType":"Trainer","artist":"Demo Artist","prices":{"market":0.04,"lowPrice":0,"lastUpdated":"2026-03-11T10:26:30.551Z"},"variants":{"PSA 10":{"marketPrice":12.34}},"imageCdnUrl":"https://tcgplayer-cdn.tcgplayer.com/product/567569_in_800x800.jpg","imageCdnUrl800":"https://tcgplayer-cdn.tcgplayer.com/product/567569_in_800x800.jpg","ebay":{"salesByGrade":{"ungraded":{"count":11,"totalValue":5809.98,"averagePrice":528.18,"medianPrice":500,"marketPrice7Day":604.99,"marketPriceMedian7Day":612.5,"smartMarketPrice":{"price":612.5,"confidence":"medium","method":"7day_filtered_weighted","daysUsed":7}},"psa10":{"count":117,"marketPrice7Day":637.48,"smartMarketPrice":{"price":597,"confidence":"high","method":"7day_filtered_weighted","daysUsed":7}}},"salesVelocity":{"dailyAverage":1.86,"weeklyAverage":13.02,"monthlyTotal":56},"totalSales":165,"totalValue":89634.91}}}`
					return testResponse(http.StatusOK, body), nil
				}
				if strings.Contains(req.URL.Path, "/api/v2/internal/card-history") && strings.Contains(req.Host, "www.pokemonpricetracker.com") {
					body := `{"data":{"population":{"totalPopulation":29247,"totalGems":23655,"combinedGemRate":80.8801,"matchConfidence":"medium","matchScore":0.6}}}`
					return testResponse(http.StatusOK, body), nil
				}
				return testResponse(http.StatusNotFound, `{"success":false}`), nil
			}),
		},
		keys: NewKeyRing([]string{"pk_live_test"}, 1000, nil),
	}
	provider := &Provider{client: client}

	snapshot, err := provider.RefreshCard(context.Background(), domain.Card{
		ID:                  "sv4a-168",
		PriceProviderCardID: "pk_test_card",
	}, domain.Set{Language: "japanese"}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if got := snapshot.PriceProviderCardID; got != "test_card" {
		t.Fatalf("expected provider card id test_card (pk_ stripped), got %q", got)
	}
	if snapshot.Ungraded == nil || snapshot.Ungraded.Amount != 0.04 {
		t.Fatalf("expected ungraded 0.04, got %#v", snapshot.Ungraded)
	}
	if snapshot.Low == nil || snapshot.Low.Amount != 0 {
		t.Fatalf("expected low 0, got %#v", snapshot.Low)
	}
	if snapshot.PSA10 == nil || snapshot.PSA10.Amount != 12.34 {
		t.Fatalf("expected psa10 12.34, got %#v", snapshot.PSA10)
	}
	if got := snapshot.CardName; got != "Defiance Band" {
		t.Fatalf("expected card name from public details, got %q", got)
	}
	if snapshot.UngradedSmartPrice == nil || snapshot.UngradedSmartPrice.Amount != 612.5 {
		t.Fatalf("expected ungraded smart price 612.5, got %#v", snapshot.UngradedSmartPrice)
	}
	if snapshot.UngradedSmartMeta == nil || snapshot.UngradedSmartMeta.Confidence != "medium" {
		t.Fatalf("expected ungraded smart meta confidence=medium, got %#v", snapshot.UngradedSmartMeta)
	}
	if snapshot.SalesVelocity == nil || snapshot.SalesVelocity.MonthlyTotal != 56 {
		t.Fatalf("expected sales velocity monthly total 56, got %#v", snapshot.SalesVelocity)
	}
	if snapshot.TotalSales != 165 {
		t.Fatalf("expected total sales 165, got %d", snapshot.TotalSales)
	}
	if snapshot.Population == nil || snapshot.Population.TotalPopulation != 29247 {
		t.Fatalf("expected population total 29247, got %#v", snapshot.Population)
	}
	if snapshot.GradeWorth == nil || snapshot.GradeWorth["psa10"].Count != 117 {
		t.Fatalf("expected psa10 grade worth count 117, got %#v", snapshot.GradeWorth)
	}
}

func TestRefreshCardAcceptsLegacyTCGPlayerID(t *testing.T) {
	client := &Client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/cards/123456/details") {
					return testResponse(http.StatusOK, `{"success":true,"data":{"tcgPlayerId":"123456","name":"X","setName":"Y","cardNumber":"1/1","prices":{"market":1.23,"lastUpdated":"2026-03-11T10:26:30.551Z"}}}`), nil
				}
				return testResponse(http.StatusNotFound, `{"success":false}`), nil
			}),
		},
		keys: nil,
	}
	provider := &Provider{client: client}

	snapshot, err := provider.RefreshCard(context.Background(), domain.Card{
		ID:          "card-1",
		TCGPlayerID: "123456",
	}, domain.Set{Language: "english"}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if got := snapshot.TCGPlayerID; got != "123456" {
		t.Fatalf("expected tcgPlayer id 123456, got %q", got)
	}
}
