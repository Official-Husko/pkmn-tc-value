package domain

import "time"

type APIKeyUsage struct {
	Fingerprint string    `json:"fingerprint"`
	Day         string    `json:"day"`
	Used        int       `json:"used"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
