package pokemonpricetracker

import (
	"encoding/json"
	"strconv"
	"strings"
)

type setsEnvelope struct {
	Success bool         `json:"success"`
	Data    []trackerSet `json:"data"`
}

type trackerSet struct {
	ID          stringValue `json:"id"`
	TCGPlayerID stringValue `json:"tcgPlayerId"`
	Name        string      `json:"name"`
	Series      string      `json:"series"`
	ReleaseDate string      `json:"releaseDate"`
	CardCount   int         `json:"cardCount"`
	Language    string      `json:"language"`
	SetCode     string      `json:"setCode"`
}

type cardsEnvelope struct {
	Success  bool            `json:"success"`
	Data     json.RawMessage `json:"data"`
	Metadata struct {
		Total      int `json:"total"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	} `json:"metadata"`
}

type trackerCard struct {
	ID             stringValue             `json:"id"`
	TCGPlayerID    stringValue             `json:"tcgPlayerId"`
	SetID          stringValue             `json:"setId"`
	SetName        string                  `json:"setName"`
	Name           string                  `json:"name"`
	CardNumber     string                  `json:"cardNumber"`
	TotalSetNumber string                  `json:"totalSetNumber"`
	Rarity         string                  `json:"rarity"`
	CardType       string                  `json:"cardType"`
	Artist         string                  `json:"artist"`
	Language       string                  `json:"language"`
	Prices         trackerPriceData        `json:"prices"`
	Variants       map[string]variantPrice `json:"variants"`
	ImageURL       string                  `json:"imageUrl"`
	ImageCdnURL    string                  `json:"imageCdnUrl"`
	ImageCdnURL200 string                  `json:"imageCdnUrl200"`
	ImageCdnURL400 string                  `json:"imageCdnUrl400"`
	ImageCdnURL800 string                  `json:"imageCdnUrl800"`
	EbayData       map[string]any          `json:"ebayData"`
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
		EbayData map[string]any `json:"ebayData"`
	} `json:"data"`
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
