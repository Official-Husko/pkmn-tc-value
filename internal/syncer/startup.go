package syncer

import (
	"context"
	"fmt"
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
			if !ok {
				stats.NewSets++
			} else if existing.Name != remote.Name || existing.Total != total || existing.ReleaseDate != remote.ReleaseDate {
				stats.UpdatedSets++
			}
			db.Sets[remote.ID] = domain.Set{
				ID:               remote.ID,
				Language:         remote.Language,
				Name:             remote.Name,
				SetCode:          remote.SetCode,
				Series:           remote.Series,
				PrintedTotal:     printedTotal,
				Total:            total,
				ReleaseDate:      remote.ReleaseDate,
				SymbolURL:        remote.SymbolURL,
				LogoURL:          remote.LogoURL,
				CatalogUpdatedAt: remote.CatalogUpdatedAt,
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
