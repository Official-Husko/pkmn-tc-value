package domain

import (
	"encoding/json"
	"time"
)

type SetCards struct {
	Total    int `json:"total"`
	Official int `json:"official"`
}

type Set struct {
	ID                   string     `json:"id"`
	Language             string     `json:"language,omitempty"`
	Name                 string     `json:"name"`
	ForeignName          string     `json:"foreign_name,omitempty"`
	EnglishName          string     `json:"-"`
	SetCode              string     `json:"setCode,omitempty"`
	PriceProviderSetID   string     `json:"-"`
	PriceProviderSetName string     `json:"-"`
	PriceProviderSetCode string     `json:"-"`
	Series               string     `json:"series"`
	Cards                SetCards   `json:"cards"`
	PrintedTotal         int        `json:"-"`
	Total                int        `json:"-"`
	ReleaseDate          string     `json:"releaseDate"`
	SymbolURL            string     `json:"symbolUrl,omitempty"`
	LogoURL              string     `json:"logoUrl,omitempty"`
	CatalogUpdatedAt     *time.Time `json:"catalogUpdatedAt,omitempty"`
}

type RemoteSet struct {
	ID                   string
	Language             string
	Name                 string
	ForeignName          string
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

func (s Set) MarshalJSON() ([]byte, error) {
	cards := s.Cards
	if cards.Total == 0 && s.Total > 0 {
		cards.Total = s.Total
	}
	if cards.Official == 0 && s.PrintedTotal > 0 {
		cards.Official = s.PrintedTotal
	}
	type outSet struct {
		ID               string     `json:"id"`
		Language         string     `json:"language,omitempty"`
		Name             string     `json:"name"`
		ForeignName      string     `json:"foreign_name,omitempty"`
		SetCode          string     `json:"setCode,omitempty"`
		Series           string     `json:"series"`
		Cards            SetCards   `json:"cards"`
		ReleaseDate      string     `json:"releaseDate"`
		SymbolURL        string     `json:"symbolUrl,omitempty"`
		LogoURL          string     `json:"logoUrl,omitempty"`
		CatalogUpdatedAt *time.Time `json:"catalogUpdatedAt,omitempty"`
	}
	return json.Marshal(outSet{
		ID:               s.ID,
		Language:         s.Language,
		Name:             s.Name,
		ForeignName:      s.ForeignName,
		SetCode:          s.SetCode,
		Series:           s.Series,
		Cards:            cards,
		ReleaseDate:      s.ReleaseDate,
		SymbolURL:        s.SymbolURL,
		LogoURL:          s.LogoURL,
		CatalogUpdatedAt: s.CatalogUpdatedAt,
	})
}

func (s *Set) UnmarshalJSON(data []byte) error {
	type inSet struct {
		ID                   string     `json:"id"`
		Language             string     `json:"language,omitempty"`
		Name                 string     `json:"name"`
		ForeignName          string     `json:"foreign_name,omitempty"`
		EnglishName          string     `json:"englishName,omitempty"`
		SetCode              string     `json:"setCode,omitempty"`
		PriceProviderSetID   string     `json:"priceProviderSetId,omitempty"`
		PriceProviderSetName string     `json:"priceProviderSetName,omitempty"`
		PriceProviderSetCode string     `json:"priceProviderSetCode,omitempty"`
		Series               string     `json:"series"`
		Cards                SetCards   `json:"cards"`
		PrintedTotal         int        `json:"printedTotal"`
		Total                int        `json:"total"`
		ReleaseDate          string     `json:"releaseDate"`
		SymbolURL            string     `json:"symbolUrl,omitempty"`
		LogoURL              string     `json:"logoUrl,omitempty"`
		CatalogUpdatedAt     *time.Time `json:"catalogUpdatedAt,omitempty"`
	}
	var in inSet
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	s.ID = in.ID
	s.Language = in.Language
	s.Name = in.Name
	s.ForeignName = in.ForeignName
	s.EnglishName = in.EnglishName
	s.SetCode = in.SetCode
	s.PriceProviderSetID = in.PriceProviderSetID
	s.PriceProviderSetName = in.PriceProviderSetName
	s.PriceProviderSetCode = in.PriceProviderSetCode
	s.Series = in.Series
	s.Cards = in.Cards
	s.ReleaseDate = in.ReleaseDate
	s.SymbolURL = in.SymbolURL
	s.LogoURL = in.LogoURL
	s.CatalogUpdatedAt = in.CatalogUpdatedAt

	if s.Cards.Total == 0 && in.Total > 0 {
		s.Cards.Total = in.Total
	}
	if s.Cards.Official == 0 && in.PrintedTotal > 0 {
		s.Cards.Official = in.PrintedTotal
	}
	s.Total = s.Cards.Total
	s.PrintedTotal = s.Cards.Official
	return nil
}
