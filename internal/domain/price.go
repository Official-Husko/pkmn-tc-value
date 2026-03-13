package domain

import "time"

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type SmartPrice struct {
	Price      *Money `json:"price,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Method     string `json:"method,omitempty"`
	DaysUsed   int    `json:"daysUsed,omitempty"`
}

type GradeWorth struct {
	Count                 int         `json:"count,omitempty"`
	TotalValue            *Money      `json:"totalValue,omitempty"`
	AveragePrice          *Money      `json:"averagePrice,omitempty"`
	MedianPrice           *Money      `json:"medianPrice,omitempty"`
	MinPrice              *Money      `json:"minPrice,omitempty"`
	MaxPrice              *Money      `json:"maxPrice,omitempty"`
	MarketPrice7Day       *Money      `json:"marketPrice7Day,omitempty"`
	MarketPriceMedian7Day *Money      `json:"marketPriceMedian7Day,omitempty"`
	DailyVolume7Day       *float64    `json:"dailyVolume7Day,omitempty"`
	MarketTrend           string      `json:"marketTrend,omitempty"`
	SmartMarketPrice      *SmartPrice `json:"smartMarketPrice,omitempty"`
}

type SalesVelocity struct {
	DailyAverage  *float64 `json:"dailyAverage,omitempty"`
	WeeklyAverage *float64 `json:"weeklyAverage,omitempty"`
	MonthlyTotal  int      `json:"monthlyTotal,omitempty"`
}

type PopulationSummary struct {
	TotalPopulation int      `json:"totalPopulation,omitempty"`
	TotalGems       int      `json:"totalGems,omitempty"`
	CombinedGemRate *float64 `json:"combinedGemRate,omitempty"`
	MatchConfidence string   `json:"matchConfidence,omitempty"`
	MatchScore      *float64 `json:"matchScore,omitempty"`
}

type SoldListing struct {
	Grade  string     `json:"grade,omitempty"`
	Title  string     `json:"title,omitempty"`
	Price  *Money     `json:"price,omitempty"`
	SoldAt *time.Time `json:"soldAt,omitempty"`
	URL    string     `json:"url,omitempty"`
}

type PriceSnapshot struct {
	Ungraded             *Money                `json:"ungraded,omitempty"`
	Low                  *Money                `json:"low,omitempty"`
	PSA10                *Money                `json:"psa10,omitempty"`
	GradeWorth           map[string]GradeWorth `json:"gradeWorth,omitempty"`
	UngradedSmartPrice   *Money                `json:"ungradedSmartPrice,omitempty"`
	UngradedSmartMeta    *SmartPrice           `json:"ungradedSmartMeta,omitempty"`
	SalesVelocity        *SalesVelocity        `json:"salesVelocity,omitempty"`
	TotalSales           int                   `json:"totalSales,omitempty"`
	TotalSalesValue      *Money                `json:"totalSalesValue,omitempty"`
	RecentSales          []SoldListing         `json:"recentSales,omitempty"`
	Population           *PopulationSummary    `json:"population,omitempty"`
	SourceURL            string                `json:"sourceUrl,omitempty"`
	CheckedAt            time.Time             `json:"checkedAt"`
	MatchedName          string                `json:"matchedName,omitempty"`
	TCGPlayerID          string                `json:"tcgPlayerId,omitempty"`
	SetName              string                `json:"setName,omitempty"`
	CardName             string                `json:"cardName,omitempty"`
	CardNumber           string                `json:"cardNumber,omitempty"`
	TotalSetNumber       string                `json:"totalSetNumber,omitempty"`
	Rarity               string                `json:"rarity,omitempty"`
	CardType             string                `json:"cardType,omitempty"`
	Artist               string                `json:"artist,omitempty"`
	ImageURL             string                `json:"imageUrl,omitempty"`
	ImageBaseURL         string                `json:"imageBaseUrl,omitempty"`
	PriceProviderCardID  string                `json:"priceProviderCardId,omitempty"`
	PriceProviderSetID   string                `json:"priceProviderSetId,omitempty"`
	PriceProviderSetName string                `json:"priceProviderSetName,omitempty"`
	PriceProviderSetCode string                `json:"priceProviderSetCode,omitempty"`
}
