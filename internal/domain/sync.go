package domain

import "time"

type SyncState struct {
	LastStartupSyncAt           *time.Time `json:"lastStartupSyncAt,omitempty"`
	LastSuccessfulStartupSyncAt *time.Time `json:"lastSuccessfulStartupSyncAt,omitempty"`
	CatalogProvider             string     `json:"catalogProvider"`
	PriceProvider               string     `json:"priceProvider"`
}
