package repository

import (
	"sort"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type SetsRepo struct {
	store *store.Store
}

func NewSetsRepo(s *store.Store) *SetsRepo {
	return &SetsRepo{store: s}
}

func (r *SetsRepo) List() ([]domain.Set, error) {
	var sets []domain.Set
	err := r.store.Read(func(db *store.DB) error {
		sets = make([]domain.Set, 0, len(db.Sets))
		for _, set := range db.Sets {
			if cached := len(db.CardsBySet[set.ID]); cached > 0 {
				set.Total = cached
			}
			sets = append(sets, set)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sets, func(i, j int) bool {
		ti := parseReleaseDate(sets[i].ReleaseDate)
		tj := parseReleaseDate(sets[j].ReleaseDate)
		if ti.Equal(tj) {
			return strings.ToLower(sets[i].Name) < strings.ToLower(sets[j].Name)
		}
		return ti.After(tj)
	})
	return sets, nil
}

func (r *SetsRepo) Get(id string) (domain.Set, bool, error) {
	var set domain.Set
	var ok bool
	err := r.store.Read(func(db *store.DB) error {
		set, ok = db.Sets[id]
		if ok {
			if cached := len(db.CardsBySet[id]); cached > 0 {
				set.Total = cached
			}
		}
		return nil
	})
	return set, ok, err
}

func parseReleaseDate(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
