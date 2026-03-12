package repository

import (
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type APIKeysRepo struct {
	store *store.Store
}

func NewAPIKeysRepo(s *store.Store) *APIKeysRepo {
	return &APIKeysRepo{store: s}
}

func (r *APIKeysRepo) UsageForDay(fingerprint string, day string) (domain.APIKeyUsage, bool, error) {
	var usage domain.APIKeyUsage
	var ok bool
	err := r.store.Read(func(db *store.DB) error {
		usage, ok = db.APIKeyUsage[fingerprint]
		if !ok || usage.Day != day {
			ok = false
		}
		return nil
	})
	return usage, ok, err
}

func (r *APIKeysRepo) SetUsage(fingerprint string, day string, used int) error {
	now := time.Now().UTC()
	return r.store.Update(func(db *store.DB) error {
		db.APIKeyUsage[fingerprint] = domain.APIKeyUsage{
			Fingerprint: fingerprint,
			Day:         day,
			Used:        used,
			UpdatedAt:   now,
		}
		return nil
	})
}

func (r *APIKeysRepo) IncrementUsage(fingerprint string, day string, delta int) (domain.APIKeyUsage, error) {
	now := time.Now().UTC()
	if delta < 0 {
		delta = 0
	}
	var out domain.APIKeyUsage
	err := r.store.Update(func(db *store.DB) error {
		usage := db.APIKeyUsage[fingerprint]
		if usage.Day != day {
			usage = domain.APIKeyUsage{
				Fingerprint: fingerprint,
				Day:         day,
				Used:        0,
			}
		}
		usage.Used += delta
		usage.UpdatedAt = now
		db.APIKeyUsage[fingerprint] = usage
		out = usage
		return nil
	})
	return out, err
}

func (r *APIKeysRepo) ListUsageForDay(day string) (map[string]domain.APIKeyUsage, error) {
	out := make(map[string]domain.APIKeyUsage)
	err := r.store.Read(func(db *store.DB) error {
		for fingerprint, usage := range db.APIKeyUsage {
			if usage.Day == day {
				out[fingerprint] = usage
			}
		}
		return nil
	})
	return out, err
}
