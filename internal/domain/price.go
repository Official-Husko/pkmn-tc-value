package domain

import "time"

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PriceSnapshot struct {
	Ungraded             *Money    `json:"ungraded,omitempty"`
	PSA10                *Money    `json:"psa10,omitempty"`
	SourceURL            string    `json:"sourceUrl,omitempty"`
	CheckedAt            time.Time `json:"checkedAt"`
	MatchedName          string    `json:"matchedName,omitempty"`
	PriceProviderCardID  string    `json:"priceProviderCardId,omitempty"`
	PriceProviderSetName string    `json:"priceProviderSetName,omitempty"`
}
