package domain

import "time"

type Card struct {
	ID                  string     `json:"id"`
	SetID               string     `json:"setId"`
	SetName             string     `json:"setName"`
	SetCode             string     `json:"setCode,omitempty"`
	PriceProviderCardID string     `json:"priceProviderCardId,omitempty"`
	Language            string     `json:"language,omitempty"`
	Name                string     `json:"name"`
	Number              string     `json:"number"`
	ReleaseDate         string     `json:"releaseDate,omitempty"`
	Secret              bool       `json:"secret"`
	TCGPlayerID         string     `json:"tcgplayerId,omitempty"`
	Rarity              string     `json:"rarity,omitempty"`
	ImageBaseURL        string     `json:"imageBaseUrl,omitempty"`
	ImageURL            string     `json:"imageUrl,omitempty"`
	ImagePath           string     `json:"imagePath,omitempty"`
	UngradedPrice       *Money     `json:"ungradedPrice,omitempty"`
	PSA10Price          *Money     `json:"psa10Price,omitempty"`
	PriceSourceURL      string     `json:"priceSourceUrl,omitempty"`
	PriceCheckedAt      *time.Time `json:"priceCheckedAt,omitempty"`
	CatalogUpdatedAt    *time.Time `json:"catalogUpdatedAt,omitempty"`
}

type RemoteCard struct {
	ID                  string
	SetID               string
	SetName             string
	SetCode             string
	PriceProviderCardID string
	Language            string
	Name                string
	Number              string
	ReleaseDate         string
	Secret              bool
	TCGPlayerID         string
	Rarity              string
	ImageBaseURL        string
	ImageURL            string
	CatalogUpdatedAt    *time.Time
}
