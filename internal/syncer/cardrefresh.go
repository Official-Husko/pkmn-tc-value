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
		card.PSA10Price = snapshot.PSA10
		card.PriceSourceURL = snapshot.SourceURL
		card.PriceCheckedAt = &snapshot.CheckedAt
		persisted.UngradedPrice = snapshot.Ungraded
		persisted.PSA10Price = snapshot.PSA10
		persisted.PriceSourceURL = snapshot.SourceURL
		persisted.PriceCheckedAt = &snapshot.CheckedAt
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
