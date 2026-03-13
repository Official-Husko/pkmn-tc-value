package pokemonpricetracker

import (
	"encoding/json"
	"strconv"
	"strings"
)

type setsEnvelope struct {
	Success bool        `json:"success"`
	Data    []walletSet `json:"data"`
}

type walletSet struct {
	Name        string      `json:"name"`
	SetCode     string      `json:"set_code"`
	SetID       stringValue `json:"set_id"`
	CardCount   int         `json:"card_count"`
	Language    string      `json:"language"`
	ReleaseDate string      `json:"release_date"`
}

type trackerSet struct {
	ID          stringValue
	TCGPlayerID stringValue
	Name        string
	Series      string
	ReleaseDate string
	CardCount   int
	Language    string
	SetCode     string
}

type cardsEnvelope struct {
	Success bool `json:"success"`
	Set     struct {
		Name       string      `json:"name"`
		SetCode    string      `json:"set_code"`
		SetID      stringValue `json:"set_id"`
		TotalCards int         `json:"total_cards"`
		Language   string      `json:"language"`
	} `json:"set"`
	Cards          []walletCard `json:"cards"`
	Disambiguation bool         `json:"disambiguation"`
	Matches        []walletSet  `json:"matches"`
}

type trackerCard struct {
	ID             stringValue
	TCGPlayerID    stringValue
	SetID          stringValue
	SetName        string
	Name           string
	CardNumber     string
	TotalSetNumber string
	Rarity         string
	CardType       string
	Artist         string
	Language       string
	Prices         trackerPriceData
	Variants       map[string]variantPrice
	ImageURL       string
	ImageCdnURL    string
	ImageCdnURL200 string
	ImageCdnURL400 string
	ImageCdnURL800 string
	Ebay           *trackerEbayData
	Population     *trackerPopulationData
	EbayData       map[string]any
}

type walletCard struct {
	ID         string          `json:"id"`
	CardInfo   walletCardInfo  `json:"card_info"`
	TCGPlayer  *walletTCGEntry `json:"tcgplayer"`
	CardMarket *walletCMEntry  `json:"cardmarket"`
}

type walletCardInfo struct {
	Name       string      `json:"name"`
	CleanName  string      `json:"clean_name"`
	SetName    string      `json:"set_name"`
	SetCode    string      `json:"set_code"`
	SetID      stringValue `json:"set_id"`
	CardNumber string      `json:"card_number"`
	TotalSet   string      `json:"total_set_number"`
	Rarity     string      `json:"rarity"`
	CardType   string      `json:"card_type"`
	HP         string      `json:"hp"`
	Stage      string      `json:"stage"`
	CardText   string      `json:"card_text"`
	Weakness   string      `json:"weakness"`
	Resistance string      `json:"resistance"`
}

type walletTCGEntry struct {
	URL    string             `json:"url"`
	Prices []walletTCGPrice   `json:"prices"`
	Raw    map[string]any     `json:"-"`
	Extra  map[string]float64 `json:"-"`
}

type walletTCGPrice struct {
	SubTypeName    string   `json:"sub_type_name"`
	LowPrice       *float64 `json:"low_price"`
	MidPrice       *float64 `json:"mid_price"`
	HighPrice      *float64 `json:"high_price"`
	MarketPrice    *float64 `json:"market_price"`
	DirectLowPrice *float64 `json:"direct_low_price"`
	UpdatedAt      string   `json:"updated_at"`
}

type walletCMEntry struct {
	ProductName string          `json:"product_name"`
	ProductURL  string          `json:"product_url"`
	Prices      []walletCMPrice `json:"prices"`
}

type walletCMPrice struct {
	Avg         *float64 `json:"avg"`
	Low         *float64 `json:"low"`
	Avg1        *float64 `json:"avg1"`
	Avg7        *float64 `json:"avg7"`
	Avg30       *float64 `json:"avg30"`
	Trend       *float64 `json:"trend"`
	UpdatedAt   string   `json:"updated_at"`
	VariantType string   `json:"variant_type"`
}

type trackerPriceData struct {
	Market      *float64 `json:"market"`
	LowPrice    *float64 `json:"lowPrice"`
	Low         *float64 `json:"low"`
	LastUpdated string   `json:"lastUpdated"`
}

type variantPrice struct {
	Printing      string   `json:"printing"`
	MarketPrice   *float64 `json:"marketPrice"`
	LowPrice      *float64 `json:"lowPrice"`
	ConditionUsed string   `json:"conditionUsed"`
}

type detailsEnvelope struct {
	Success bool        `json:"success"`
	Data    trackerCard `json:"data"`
}

type historyEnvelope struct {
	Data struct {
		Ebay       *trackerEbayData       `json:"ebay"`
		Population *trackerPopulationData `json:"population"`
		EbayData   map[string]any         `json:"ebayData"`
	} `json:"data"`
}

type trackerEbayData struct {
	SalesByGrade  map[string]trackerGradeSales `json:"salesByGrade"`
	SoldListings  map[string][]trackerListing  `json:"soldListings"`
	SalesVelocity trackerSalesVelocity         `json:"salesVelocity"`
	TotalSales    int                          `json:"totalSales"`
	TotalValue    *float64                     `json:"totalValue"`
}

type trackerListing struct {
	Title          string      `json:"title"`
	Price          *float64    `json:"price"`
	SoldDate       string      `json:"soldDate"`
	URL            string      `json:"url"`
	Currency       string      `json:"currency"`
	GradingCompany string      `json:"gradingCompany"`
	Grade          interface{} `json:"grade"`
}

type trackerSalesVelocity struct {
	DailyAverage  *float64 `json:"dailyAverage"`
	WeeklyAverage *float64 `json:"weeklyAverage"`
	MonthlyTotal  int      `json:"monthlyTotal"`
}

type trackerGradeSales struct {
	Count                 int                      `json:"count"`
	TotalValue            *float64                 `json:"totalValue"`
	AveragePrice          *float64                 `json:"averagePrice"`
	MedianPrice           *float64                 `json:"medianPrice"`
	MinPrice              *float64                 `json:"minPrice"`
	MaxPrice              *float64                 `json:"maxPrice"`
	MarketPrice7Day       *float64                 `json:"marketPrice7Day"`
	MarketPriceMedian7Day *float64                 `json:"marketPriceMedian7Day"`
	DailyVolume7Day       *float64                 `json:"dailyVolume7Day"`
	MarketTrend           string                   `json:"marketTrend"`
	SmartMarketPrice      *trackerSmartMarketPrice `json:"smartMarketPrice"`
}

type trackerSmartMarketPrice struct {
	Price      *float64 `json:"price"`
	Confidence string   `json:"confidence"`
	Method     string   `json:"method"`
	DaysUsed   int      `json:"daysUsed"`
}

type trackerPopulationData struct {
	TotalPopulation int      `json:"totalPopulation"`
	TotalGems       int      `json:"totalGems"`
	CombinedGemRate *float64 `json:"combinedGemRate"`
	MatchConfidence string   `json:"matchConfidence"`
	MatchScore      *float64 `json:"matchScore"`
}

type stringValue string

func (s *stringValue) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*s = ""
		return nil
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = stringValue(strings.TrimSpace(value))
		return nil
	}
	var intValue int64
	if err := json.Unmarshal(data, &intValue); err == nil {
		*s = stringValue(strconv.FormatInt(intValue, 10))
		return nil
	}
	var floatValue float64
	if err := json.Unmarshal(data, &floatValue); err == nil {
		*s = stringValue(strconv.FormatFloat(floatValue, 'f', -1, 64))
		return nil
	}
	return nil
}

func (s stringValue) String() string {
	return strings.TrimSpace(string(s))
}
