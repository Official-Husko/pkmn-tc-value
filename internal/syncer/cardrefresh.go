package syncer

import (
	"context"
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
	snapshot, err := s.pricing.RefreshCard(ctx, card, set, cfg)
	if err != nil {
		return card, err
	}
	if cfg.ImagePreviewsEnabled && card.ImagePath == "" && card.ImageURL != "" {
		if path, imgErr := s.images.Ensure(ctx, card); imgErr == nil {
			card.ImagePath = path
		}
	}
	card.UngradedPrice = snapshot.Ungraded
	card.PSA10Price = snapshot.PSA10
	card.PriceSourceURL = snapshot.SourceURL
	card.PriceCheckedAt = &snapshot.CheckedAt

	err = s.store.Update(func(db *store.DB) error {
		if db.CardsBySet[card.SetID] == nil {
			db.CardsBySet[card.SetID] = make(map[string]domain.Card)
		}
		db.CardsBySet[card.SetID][card.ID] = card
		return nil
	})
	return card, err
}
