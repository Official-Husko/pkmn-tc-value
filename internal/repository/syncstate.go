package repository

import (
	"strings"
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

func (r *SyncStateRepo) SetLastViewedSetID(setID string) error {
	id := strings.TrimSpace(setID)
	return r.store.Update(func(db *store.DB) error {
		db.SyncState.LastViewedSetID = id
		return nil
	})
}

func (r *SyncStateRepo) LastViewedSetID() (string, error) {
	var id string
	err := r.store.Read(func(db *store.DB) error {
		id = strings.TrimSpace(db.SyncState.LastViewedSetID)
		return nil
	})
	return id, err
}
