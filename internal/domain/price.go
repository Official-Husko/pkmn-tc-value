package domain

import "time"

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PriceSnapshot struct {
	Ungraded             *Money    `json:"ungraded,omitempty"`
	Low                  *Money    `json:"low,omitempty"`
	PSA10                *Money    `json:"psa10,omitempty"`
	SourceURL            string    `json:"sourceUrl,omitempty"`
	CheckedAt            time.Time `json:"checkedAt"`
	MatchedName          string    `json:"matchedName,omitempty"`
	TCGPlayerID          string    `json:"tcgPlayerId,omitempty"`
	SetName              string    `json:"setName,omitempty"`
	CardName             string    `json:"cardName,omitempty"`
	CardNumber           string    `json:"cardNumber,omitempty"`
	TotalSetNumber       string    `json:"totalSetNumber,omitempty"`
	Rarity               string    `json:"rarity,omitempty"`
	CardType             string    `json:"cardType,omitempty"`
	Artist               string    `json:"artist,omitempty"`
	ImageURL             string    `json:"imageUrl,omitempty"`
	ImageBaseURL         string    `json:"imageBaseUrl,omitempty"`
	PriceProviderCardID  string    `json:"priceProviderCardId,omitempty"`
	PriceProviderSetID   string    `json:"priceProviderSetId,omitempty"`
	PriceProviderSetName string    `json:"priceProviderSetName,omitempty"`
	PriceProviderSetCode string    `json:"priceProviderSetCode,omitempty"`
}
