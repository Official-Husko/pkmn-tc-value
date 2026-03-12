package domain

import "time"

type Set struct {
	ID                   string     `json:"id"`
	Language             string     `json:"language,omitempty"`
	Name                 string     `json:"name"`
	EnglishName          string     `json:"englishName,omitempty"`
	SetCode              string     `json:"setCode,omitempty"`
	PriceProviderSetID   string     `json:"priceProviderSetId,omitempty"`
	PriceProviderSetName string     `json:"priceProviderSetName,omitempty"`
	PriceProviderSetCode string     `json:"priceProviderSetCode,omitempty"`
	Series               string     `json:"series"`
	PrintedTotal         int        `json:"printedTotal"`
	Total                int        `json:"total"`
	ReleaseDate          string     `json:"releaseDate"`
	SymbolURL            string     `json:"symbolUrl,omitempty"`
	LogoURL              string     `json:"logoUrl,omitempty"`
	CatalogUpdatedAt     *time.Time `json:"catalogUpdatedAt,omitempty"`
}

type RemoteSet struct {
	ID                   string
	Language             string
	Name                 string
	EnglishName          string
	SetCode              string
	PriceProviderSetID   string
	PriceProviderSetName string
	PriceProviderSetCode string
	Series               string
	PrintedTotal         int
	Total                int
	ReleaseDate          string
	SymbolURL            string
	LogoURL              string
	CatalogUpdatedAt     *time.Time
}
