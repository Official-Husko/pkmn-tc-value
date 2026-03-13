package domain

import "time"

type Card struct {
	ID                  string                `json:"id"`
	SetID               string                `json:"setId"`
	SetName             string                `json:"-"`
	SetEnglishName      string                `json:"-"`
	SetCode             string                `json:"setCode,omitempty"`
	PriceProviderSetID  string                `json:"priceProviderSetId,omitempty"`
	PriceProviderCardID string                `json:"priceProviderCardId,omitempty"`
	Language            string                `json:"language,omitempty"`
	Name                string                `json:"name"`
	EnglishName         string                `json:"englishName,omitempty"`
	Number              string                `json:"number"`
	TotalSetNumber      string                `json:"-"`
	ReleaseDate         string                `json:"releaseDate,omitempty"`
	Secret              bool                  `json:"secret"`
	TCGPlayerID         string                `json:"tcgplayerId,omitempty"`
	Rarity              string                `json:"rarity,omitempty"`
	CardType            string                `json:"cardType,omitempty"`
	HP                  string                `json:"hp,omitempty"`
	Stage               string                `json:"stage,omitempty"`
	CardText            string                `json:"cardText,omitempty"`
	Attacks             []string              `json:"attacks,omitempty"`
	Weakness            string                `json:"weakness,omitempty"`
	Resistance          string                `json:"resistance,omitempty"`
	RetreatCost         string                `json:"retreatCost,omitempty"`
	Artist              string                `json:"artist,omitempty"`
	ImageBaseURL        string                `json:"imageBaseUrl,omitempty"`
	ImageURL            string                `json:"imageUrl,omitempty"`
	ImagePath           string                `json:"imagePath,omitempty"`
	UngradedPrice       *Money                `json:"ungradedPrice,omitempty"`
	LowPrice            *Money                `json:"lowPrice,omitempty"`
	PSA10Price          *Money                `json:"psa10Price,omitempty"`
	GradeWorth          map[string]GradeWorth `json:"gradeWorth,omitempty"`
	UngradedSmartPrice  *Money                `json:"ungradedSmartPrice,omitempty"`
	UngradedSmartMeta   *SmartPrice           `json:"ungradedSmartMeta,omitempty"`
	SalesVelocity       *SalesVelocity        `json:"salesVelocity,omitempty"`
	TotalSales          int                   `json:"totalSales,omitempty"`
	TotalSalesValue     *Money                `json:"totalSalesValue,omitempty"`
	RecentSales         []SoldListing         `json:"recentSales,omitempty"`
	Population          *PopulationSummary    `json:"population,omitempty"`
	PriceSourceURL      string                `json:"priceSourceUrl,omitempty"`
	PriceCheckedAt      *time.Time            `json:"priceCheckedAt,omitempty"`
	CatalogUpdatedAt    *time.Time            `json:"catalogUpdatedAt,omitempty"`
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
	HP                  string
	Stage               string
	CardText            string
	Attacks             []string
	Weakness            string
	Resistance          string
	RetreatCost         string
	Artist              string
	ImageBaseURL        string
	ImageURL            string
	CatalogUpdatedAt    *time.Time
}
