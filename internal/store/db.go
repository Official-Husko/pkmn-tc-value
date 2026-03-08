package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	if db.Meta.SchemaVersion != SchemaVersion {
		_ = backupLegacyDB(path, data, db.Meta.SchemaVersion)
		return &Store{path: path, db: NewDB()}, nil
	}
	db.ensureMaps()
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
	data, err := marshalIndentNoEscapeHTML(s.db)
	if err != nil {
		return fmt.Errorf("encode database: %w", err)
	}
	return WriteFileAtomically(s.path, data, 0o600)
}

func marshalIndentNoEscapeHTML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func backupLegacyDB(path string, data []byte, schemaVersion int) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupName := fmt.Sprintf("%s.schema-%d.%s.bak", base, schemaVersion, timestamp)
	backupPath := filepath.Join(dir, backupName)
	return WriteFileAtomically(backupPath, data, 0o600)
}
