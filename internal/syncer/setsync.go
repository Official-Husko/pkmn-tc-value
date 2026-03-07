package syncer

import (
	"context"
	"fmt"
	"sort"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

type SetSyncResult struct {
	SetName       string
	SetID         string
	NewCards      int
	UpdatedCards  int
	TotalCards    int
	ImagesSaved   int
	DetailsSynced int
	DetailsFailed int
}

type SetSyncOptions struct {
	SaveCardImages  bool
	SyncCardDetails bool
	Config          config.Config
}

type SetSyncProgress struct {
	Stage  string
	Status string
	Done   int
	Total  int
}

type SetSyncService struct {
	store   *store.Store
	catalog catalog.Provider
	pricing pricing.Provider
	images  *images.Downloader
}

func NewSetSyncService(s *store.Store, c catalog.Provider, p pricing.Provider, i *images.Downloader) *SetSyncService {
	return &SetSyncService{store: s, catalog: c, pricing: p, images: i}
}

func (s *SetSyncService) IsSetCached(setID string) (bool, error) {
	var count int
	err := s.store.Read(func(db *store.DB) error {
		count = len(db.CardsBySet[setID])
		return nil
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SetSyncService) SyncSet(ctx context.Context, setID string, opts SetSyncOptions, progress func(SetSyncProgress)) (SetSyncResult, error) {
	progress = withDefaultProgress(progress)

	progress(SetSyncProgress{Stage: "cards", Status: "Fetching cards", Done: 0, Total: 0})
	var set domain.Set
	found := false
	if err := s.store.Read(func(db *store.DB) error {
		set, found = db.Sets[setID]
		return nil
	}); err != nil {
		return SetSyncResult{}, err
	}
	if !found {
		return SetSyncResult{}, fmt.Errorf("set %s not found", setID)
	}

	remoteCards, err := s.catalog.FetchCardsForSet(ctx, setID)
	if err != nil {
		return SetSyncResult{}, err
	}

	result := SetSyncResult{
		SetID:      setID,
		SetName:    set.Name,
		TotalCards: len(remoteCards),
	}
	progress(SetSyncProgress{Stage: "cards", Status: fmt.Sprintf("Fetched %d cards", len(remoteCards)), Done: len(remoteCards), Total: len(remoteCards)})

	var existingCards map[string]domain.Card
	if err := s.store.Read(func(db *store.DB) error {
		existingCards = db.CardsBySet[setID]
		return nil
	}); err != nil {
		return SetSyncResult{}, err
	}
	if existingCards == nil {
		existingCards = make(map[string]domain.Card)
	}

	nextCards := make(map[string]domain.Card, len(remoteCards))
	for _, remoteCard := range remoteCards {
		remoteCanonical := util.NormalizeCardNumber(remoteCard.Number)
		existing, ok := existingCards[remoteCard.ID]
		if !ok {
			for _, candidate := range existingCards {
				if util.NormalizeCardNumber(candidate.Number) == remoteCanonical {
					existing = candidate
					break
				}
			}
		}
		if existing.ID == "" {
			result.NewCards++
		} else if existing.Name != remoteCard.Name || existing.Rarity != remoteCard.Rarity || existing.ImageURL != remoteCard.ImageURL {
			result.UpdatedCards++
		}
		nextCards[remoteCard.ID] = domain.Card{
			ID:               remoteCard.ID,
			SetID:            remoteCard.SetID,
			SetName:          remoteCard.SetName,
			SetCode:          remoteCard.SetCode,
			Language:         remoteCard.Language,
			Name:             remoteCard.Name,
			Number:           remoteCard.Number,
			ReleaseDate:      remoteCard.ReleaseDate,
			Secret:           remoteCard.Secret,
			TCGPlayerID:      remoteCard.TCGPlayerID,
			Rarity:           remoteCard.Rarity,
			ImageURL:         remoteCard.ImageURL,
			ImagePath:        existing.ImagePath,
			UngradedPrice:    existing.UngradedPrice,
			PSA10Price:       existing.PSA10Price,
			PriceSourceURL:   existing.PriceSourceURL,
			PriceCheckedAt:   existing.PriceCheckedAt,
			CatalogUpdatedAt: remoteCard.CatalogUpdatedAt,
		}
	}

	orderedIDs := make([]string, 0, len(nextCards))
	for cardID := range nextCards {
		orderedIDs = append(orderedIDs, cardID)
	}
	sort.Strings(orderedIDs)

	if opts.SaveCardImages && s.images != nil {
		total := len(orderedIDs)
		done := 0
		progress(SetSyncProgress{Stage: "images", Status: "Downloading card images", Done: done, Total: total})
		for _, cardID := range orderedIDs {
			done++
			card := nextCards[cardID]
			path, err := s.images.Ensure(ctx, card)
			if err == nil && path != "" && card.ImagePath != path {
				card.ImagePath = path
				nextCards[cardID] = card
				result.ImagesSaved++
			}
			progress(SetSyncProgress{
				Stage:  "images",
				Status: fmt.Sprintf("Downloading card images (%d/%d)", done, total),
				Done:   done,
				Total:  total,
			})
		}
	}

	if opts.SyncCardDetails && s.pricing != nil {
		total := len(orderedIDs)
		done := 0
		progress(SetSyncProgress{Stage: "details", Status: "Downloading card details", Done: done, Total: total})
		for _, cardID := range orderedIDs {
			done++
			card := nextCards[cardID]
			snapshot, err := s.pricing.RefreshCard(ctx, card, set, opts.Config)
			if err != nil {
				result.DetailsFailed++
			} else {
				card.UngradedPrice = snapshot.Ungraded
				card.PSA10Price = snapshot.PSA10
				card.PriceSourceURL = snapshot.SourceURL
				card.PriceCheckedAt = &snapshot.CheckedAt
				nextCards[cardID] = card
				result.DetailsSynced++
			}
			progress(SetSyncProgress{
				Stage:  "details",
				Status: fmt.Sprintf("Downloading card details (%d/%d)", done, total),
				Done:   done,
				Total:  total,
			})
		}
	}

	err = s.store.Update(func(db *store.DB) error {
		db.CardsBySet[setID] = nextCards
		setRecord := db.Sets[setID]
		setRecord.Total = len(nextCards)
		db.Sets[setID] = setRecord
		return nil
	})
	return result, err
}

func withDefaultProgress(progress func(SetSyncProgress)) func(SetSyncProgress) {
	if progress == nil {
		return func(SetSyncProgress) {}
	}
	return progress
}
