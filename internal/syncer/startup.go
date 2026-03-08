package syncer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type StartupService struct {
	store   *store.Store
	catalog catalog.Provider
}

func NewStartupService(s *store.Store, c catalog.Provider) *StartupService {
	return &StartupService{store: s, catalog: c}
}

func (s *StartupService) Run(ctx context.Context, progress func(StartupProgress)) (Stats, error) {
	progress(StartupProgress{Stage: "sets", Status: "Fetching set list"})
	remoteSets, err := s.catalog.FetchSets(ctx)
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{}
	now := time.Now().UTC()

	progress(StartupProgress{
		Stage:      "sets",
		Status:     fmt.Sprintf("Fetched %d sets", len(remoteSets)),
		SetsTotal:  len(remoteSets),
		CardsTotal: 0,
	})

	err = s.store.Update(func(db *store.DB) error {
		for _, remote := range remoteSets {
			existing, ok := db.Sets[remote.ID]
			total := remote.Total
			if cached := len(db.CardsBySet[remote.ID]); cached > 0 {
				total = cached
			}
			if total == 0 && ok {
				total = existing.Total
			}
			printedTotal := remote.PrintedTotal
			if printedTotal == 0 && ok {
				printedTotal = existing.PrintedTotal
			}
			setCode := strings.TrimSpace(remote.SetCode)
			if setCode == "" && ok {
				setCode = strings.TrimSpace(existing.SetCode)
			}
			priceProviderSetName := strings.TrimSpace(remote.PriceProviderSetName)
			if priceProviderSetName == "" && ok {
				priceProviderSetName = strings.TrimSpace(existing.PriceProviderSetName)
			}
			if !ok {
				stats.NewSets++
			} else if existing.Name != remote.Name || existing.Total != total || existing.ReleaseDate != remote.ReleaseDate {
				stats.UpdatedSets++
			}
			db.Sets[remote.ID] = domain.Set{
				ID:                   remote.ID,
				Language:             remote.Language,
				Name:                 remote.Name,
				SetCode:              setCode,
				PriceProviderSetName: priceProviderSetName,
				Series:               remote.Series,
				PrintedTotal:         printedTotal,
				Total:                total,
				ReleaseDate:          remote.ReleaseDate,
				SymbolURL:            remote.SymbolURL,
				LogoURL:              remote.LogoURL,
				CatalogUpdatedAt:     remote.CatalogUpdatedAt,
			}

			if setCode != "" {
				if cards, ok := db.CardsBySet[remote.ID]; ok {
					for cardID, card := range cards {
						updated := false
						if strings.TrimSpace(card.SetCode) == "" {
							card.SetCode = setCode
							updated = true
						}
						if strings.TrimSpace(card.SetName) == "" && strings.TrimSpace(remote.Name) != "" {
							card.SetName = remote.Name
							updated = true
						}
						if strings.TrimSpace(card.Language) == "" && strings.TrimSpace(remote.Language) != "" {
							card.Language = remote.Language
							updated = true
						}
						if updated {
							cards[cardID] = card
						}
					}
				}
			}
		}
		db.SyncState.LastStartupSyncAt = &now
		db.SyncState.CatalogProvider = s.catalog.Name()
		return nil
	})
	if err != nil {
		return Stats{}, err
	}

	err = s.store.Update(func(db *store.DB) error {
		db.SyncState.LastSuccessfulStartupSyncAt = &now
		return nil
	})
	return stats, err
}
