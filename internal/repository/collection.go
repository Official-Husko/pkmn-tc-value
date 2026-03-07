package repository

import (
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type CollectionRepo struct {
	store *store.Store
}

func NewCollectionRepo(s *store.Store) *CollectionRepo {
	return &CollectionRepo{store: s}
}

func (r *CollectionRepo) Add(cardID string) error {
	now := time.Now().UTC()
	return r.store.Update(func(db *store.DB) error {
		entry, ok := db.Collection[cardID]
		if !ok {
			db.Collection[cardID] = domain.CollectionEntry{
				CardID:    cardID,
				Quantity:  1,
				CreatedAt: now,
				UpdatedAt: now,
			}
			return nil
		}
		entry.Quantity++
		entry.UpdatedAt = now
		db.Collection[cardID] = entry
		return nil
	})
}
