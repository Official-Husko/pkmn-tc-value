package store

import (
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

const SchemaVersion = 3

type Meta struct {
	SchemaVersion int       `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type DB struct {
	Meta        Meta                              `json:"meta"`
	SyncState   domain.SyncState                  `json:"syncState"`
	APIKeyUsage map[string]domain.APIKeyUsage     `json:"apiKeyUsage"`
	Sets        map[string]domain.Set             `json:"sets"`
	CardsBySet  map[string]map[string]domain.Card `json:"cardsBySet"`
	Collection  map[string]domain.CollectionEntry `json:"collection"`
}

func NewDB() *DB {
	now := time.Now().UTC()
	return &DB{
		Meta: Meta{
			SchemaVersion: SchemaVersion,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		SyncState: domain.SyncState{
			CatalogProvider: "tcgdex",
			PriceProvider:   "pokewallet",
		},
		APIKeyUsage: make(map[string]domain.APIKeyUsage),
		Sets:        make(map[string]domain.Set),
		CardsBySet:  make(map[string]map[string]domain.Card),
		Collection:  make(map[string]domain.CollectionEntry),
	}
}

func (db *DB) ensureMaps() {
	if db.APIKeyUsage == nil {
		db.APIKeyUsage = make(map[string]domain.APIKeyUsage)
	}
	if db.Sets == nil {
		db.Sets = make(map[string]domain.Set)
	}
	if db.CardsBySet == nil {
		db.CardsBySet = make(map[string]map[string]domain.Card)
	}
	if db.Collection == nil {
		db.Collection = make(map[string]domain.CollectionEntry)
	}
}
