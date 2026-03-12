package domain

import "time"

type Card struct {
	ID                  string     `json:"id"`
	SetID               string     `json:"setId"`
	SetName             string     `json:"setName"`
	SetEnglishName      string     `json:"setEnglishName,omitempty"`
	SetCode             string     `json:"setCode,omitempty"`
	PriceProviderSetID  string     `json:"priceProviderSetId,omitempty"`
	PriceProviderCardID string     `json:"priceProviderCardId,omitempty"`
	Language            string     `json:"language,omitempty"`
	Name                string     `json:"name"`
	EnglishName         string     `json:"englishName,omitempty"`
	Number              string     `json:"number"`
	TotalSetNumber      string     `json:"totalSetNumber,omitempty"`
	ReleaseDate         string     `json:"releaseDate,omitempty"`
	Secret              bool       `json:"secret"`
	TCGPlayerID         string     `json:"tcgplayerId,omitempty"`
	Rarity              string     `json:"rarity,omitempty"`
	CardType            string     `json:"cardType,omitempty"`
	Artist              string     `json:"artist,omitempty"`
	ImageBaseURL        string     `json:"imageBaseUrl,omitempty"`
	ImageURL            string     `json:"imageUrl,omitempty"`
	ImagePath           string     `json:"imagePath,omitempty"`
	UngradedPrice       *Money     `json:"ungradedPrice,omitempty"`
	LowPrice            *Money     `json:"lowPrice,omitempty"`
	PSA10Price          *Money     `json:"psa10Price,omitempty"`
	PriceSourceURL      string     `json:"priceSourceUrl,omitempty"`
	PriceCheckedAt      *time.Time `json:"priceCheckedAt,omitempty"`
	CatalogUpdatedAt    *time.Time `json:"catalogUpdatedAt,omitempty"`
}

type RemoteCard struct {
	ID                  string
	SetID               string
	SetName             string
	SetEnglishName      string
	SetCode             string
	PriceProviderSetID  string
	PriceProviderCardID string
	Language            string
	Name                string
	EnglishName         string
	Number              string
	TotalSetNumber      string
	ReleaseDate         string
	Secret              bool
	TCGPlayerID         string
	Rarity              string
	CardType            string
	Artist              string
	ImageBaseURL        string
	ImageURL            string
	CatalogUpdatedAt    *time.Time
}
