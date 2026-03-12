package syncer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type CardRefreshService struct {
	store   *store.Store
	pricing pricing.Provider
	images  *images.Downloader
}

func NewCardRefreshService(s *store.Store, p pricing.Provider, i *images.Downloader) *CardRefreshService {
	return &CardRefreshService{store: s, pricing: p, images: i}
}

func (s *CardRefreshService) NeedsRefresh(card domain.Card, cfg config.Config) bool {
	if card.PriceCheckedAt == nil {
		return true
	}
	ttl := time.Duration(cfg.CardRefreshTTLHours) * time.Hour
	return time.Since(*card.PriceCheckedAt) > ttl
}

func (s *CardRefreshService) Refresh(ctx context.Context, card domain.Card, set domain.Set, cfg config.Config) (domain.Card, error) {
	persisted := card

	priceNeedsRefresh := s.NeedsRefresh(card, cfg)
	if priceNeedsRefresh {
		snapshot, err := s.pricing.RefreshCard(ctx, card, set, cfg)
		if err != nil {
			return card, err
		}
		card.UngradedPrice = snapshot.Ungraded
		card.LowPrice = snapshot.Low
		card.PSA10Price = snapshot.PSA10
		card.PriceSourceURL = snapshot.SourceURL
		card.PriceCheckedAt = &snapshot.CheckedAt
		persisted.UngradedPrice = snapshot.Ungraded
		persisted.LowPrice = snapshot.Low
		persisted.PSA10Price = snapshot.PSA10
		persisted.PriceSourceURL = snapshot.SourceURL
		persisted.PriceCheckedAt = &snapshot.CheckedAt
		if strings.TrimSpace(snapshot.PriceProviderCardID) != "" {
			card.PriceProviderCardID = snapshot.PriceProviderCardID
			persisted.PriceProviderCardID = snapshot.PriceProviderCardID
		}
		if strings.TrimSpace(snapshot.TCGPlayerID) != "" {
			card.TCGPlayerID = snapshot.TCGPlayerID
			persisted.TCGPlayerID = snapshot.TCGPlayerID
			if strings.TrimSpace(card.PriceProviderCardID) == "" {
				card.PriceProviderCardID = snapshot.TCGPlayerID
				persisted.PriceProviderCardID = snapshot.TCGPlayerID
			}
		}
		if strings.TrimSpace(snapshot.SetName) != "" {
			card.SetName = snapshot.SetName
			persisted.SetName = snapshot.SetName
		}
		if strings.TrimSpace(snapshot.CardName) != "" {
			card.Name = snapshot.CardName
			persisted.Name = snapshot.CardName
		}
		if strings.TrimSpace(snapshot.CardNumber) != "" {
			card.Number = snapshot.CardNumber
			persisted.Number = snapshot.CardNumber
		}
		if strings.TrimSpace(snapshot.TotalSetNumber) != "" {
			card.TotalSetNumber = snapshot.TotalSetNumber
			persisted.TotalSetNumber = snapshot.TotalSetNumber
		}
		if strings.TrimSpace(snapshot.Rarity) != "" {
			card.Rarity = snapshot.Rarity
			persisted.Rarity = snapshot.Rarity
		}
		if strings.TrimSpace(snapshot.CardType) != "" {
			card.CardType = snapshot.CardType
			persisted.CardType = snapshot.CardType
		}
		if strings.TrimSpace(snapshot.Artist) != "" {
			card.Artist = snapshot.Artist
			persisted.Artist = snapshot.Artist
		}
		if strings.TrimSpace(snapshot.ImageURL) != "" {
			card.ImageURL = snapshot.ImageURL
			persisted.ImageURL = snapshot.ImageURL
		}
		if strings.TrimSpace(snapshot.ImageBaseURL) != "" {
			card.ImageBaseURL = snapshot.ImageBaseURL
			persisted.ImageBaseURL = snapshot.ImageBaseURL
		}
		if strings.TrimSpace(snapshot.PriceProviderSetName) != "" {
			set.PriceProviderSetName = snapshot.PriceProviderSetName
		}
		if strings.TrimSpace(snapshot.PriceProviderSetID) != "" {
			set.PriceProviderSetID = snapshot.PriceProviderSetID
		}
		if strings.TrimSpace(snapshot.PriceProviderSetCode) != "" {
			set.PriceProviderSetCode = snapshot.PriceProviderSetCode
		}
	}
	if strings.TrimSpace(card.SetCode) == "" {
		card.SetCode = set.SetCode
		persisted.SetCode = set.SetCode
	}
	if strings.TrimSpace(card.SetName) == "" {
		card.SetName = set.Name
		persisted.SetName = set.Name
	}
	if strings.TrimSpace(card.Language) == "" {
		card.Language = set.Language
		persisted.Language = set.Language
	}

	if cfg.ImagePreviewsEnabled && shouldFetchImage(card) {
		if cfg.ImageCaching {
			if path, imgErr := s.images.Ensure(ctx, card); imgErr == nil {
				card.ImagePath = path
				persisted.ImagePath = path
			}
		} else {
			if path, imgErr := s.images.FetchTempPNG(ctx, card); imgErr == nil {
				card.ImagePath = path
			}
		}
	}

	err := s.store.Update(func(db *store.DB) error {
		if db.CardsBySet[persisted.SetID] == nil {
			db.CardsBySet[persisted.SetID] = make(map[string]domain.Card)
		}
		db.CardsBySet[persisted.SetID][persisted.ID] = persisted
		if currentSet, ok := db.Sets[persisted.SetID]; ok {
			if strings.TrimSpace(currentSet.SetCode) == "" && strings.TrimSpace(set.SetCode) != "" {
				currentSet.SetCode = set.SetCode
			}
			if strings.TrimSpace(currentSet.PriceProviderSetName) == "" && strings.TrimSpace(set.PriceProviderSetName) != "" {
				currentSet.PriceProviderSetName = set.PriceProviderSetName
			}
			if strings.TrimSpace(currentSet.PriceProviderSetID) == "" && strings.TrimSpace(set.PriceProviderSetID) != "" {
				currentSet.PriceProviderSetID = set.PriceProviderSetID
			}
			if strings.TrimSpace(currentSet.PriceProviderSetCode) == "" && strings.TrimSpace(set.PriceProviderSetCode) != "" {
				currentSet.PriceProviderSetCode = set.PriceProviderSetCode
			}
			db.Sets[persisted.SetID] = currentSet
		}
		return nil
	})
	return card, err
}

func shouldFetchImage(card domain.Card) bool {
	if strings.TrimSpace(card.Number) == "" {
		return false
	}
	if card.ImagePath == "" {
		return true
	}
	if strings.ToLower(filepath.Ext(card.ImagePath)) != ".png" {
		return true
	}
	_, err := os.Stat(card.ImagePath)
	return err != nil
}
