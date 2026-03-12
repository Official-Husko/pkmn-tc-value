package pokemonpricetracker

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestRefreshCardUsesPublicDetailsWithTCGPlayerID(t *testing.T) {
	client := &Client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/api/cards/567569/details") {
					body := `{"success":true,"data":{"tcgPlayerId":"567569","setName":"SV4a: Shiny Treasure ex","name":"Defiance Band","cardNumber":"168/190","rarity":"None","cardType":"Trainer","artist":"Demo Artist","prices":{"market":0.04,"lowPrice":0,"lastUpdated":"2026-03-11T10:26:30.551Z"},"variants":{"PSA 10":{"marketPrice":12.34}},"imageCdnUrl":"https://tcgplayer-cdn.tcgplayer.com/product/567569_in_800x800.jpg","imageCdnUrl800":"https://tcgplayer-cdn.tcgplayer.com/product/567569_in_800x800.jpg"}}`
					return testResponse(http.StatusOK, body), nil
				}
				return testResponse(http.StatusNotFound, `{"success":false}`), nil
			}),
		},
		keys: nil,
	}
	provider := &Provider{client: client}

	snapshot, err := provider.RefreshCard(context.Background(), domain.Card{
		ID:          "sv4a-168",
		TCGPlayerID: "567569",
	}, domain.Set{Language: "japanese"}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if got := snapshot.PriceProviderCardID; got != "567569" {
		t.Fatalf("expected provider card id 567569, got %q", got)
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
}

func TestRefreshCardAcceptsLegacyProviderCardID(t *testing.T) {
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
		ID:                  "card-1",
		PriceProviderCardID: "123456",
	}, domain.Set{Language: "english"}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if got := snapshot.TCGPlayerID; got != "123456" {
		t.Fatalf("expected tcgPlayer id 123456, got %q", got)
	}
}
