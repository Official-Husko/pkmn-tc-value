package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Store struct {
	mu   sync.RWMutex
	path string
	db   *DB
}

func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{path: path, db: NewDB()}, nil
		}
		return nil, fmt.Errorf("read database: %w", err)
	}
	db := NewDB()
	if err := json.Unmarshal(data, db); err != nil {
		return nil, fmt.Errorf("decode database: %w", err)
	}
	db.ensureMaps()
	if db.Meta.SchemaVersion == 0 {
		db.Meta.SchemaVersion = SchemaVersion
	}
	return &Store{path: path, db: db}, nil
}

func (s *Store) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.db.Sets) > 0 || len(s.db.CardsBySet) > 0
}

func (s *Store) Read(fn func(*DB) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(s.db)
}

func (s *Store) Update(fn func(*DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.db); err != nil {
		return err
	}
	s.db.ensureMaps()
	s.db.Meta.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return fmt.Errorf("encode database: %w", err)
	}
	return WriteFileAtomically(s.path, data, 0o600)
}
