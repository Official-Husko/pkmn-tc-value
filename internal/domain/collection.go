package domain

import "time"

type CollectionEntry struct {
	CardID    string    `json:"cardId"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
