package repository

import (
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type SyncStateRepo struct {
	store *store.Store
}

func NewSyncStateRepo(s *store.Store) *SyncStateRepo {
	return &SyncStateRepo{store: s}
}

func (r *SyncStateRepo) TouchStartup(success bool, catalogProvider, priceProvider string) error {
	now := time.Now().UTC()
	return r.store.Update(func(db *store.DB) error {
		db.SyncState.LastStartupSyncAt = &now
		db.SyncState.CatalogProvider = catalogProvider
		db.SyncState.PriceProvider = priceProvider
		if success {
			db.SyncState.LastSuccessfulStartupSyncAt = &now
		}
		return nil
	})
}
