package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

type Store struct {
	mu             sync.RWMutex
	mainPath       string
	setsPath       string
	cardsPath      string
	collectionPath string
	db             *DB
}

type mainFileData struct {
	Meta       Meta                              `json:"meta"`
	SyncState  domain.SyncState                  `json:"syncState"`
	Collection map[string]domain.CollectionEntry `json:"collection,omitempty"`
	Sets       map[string]domain.Set             `json:"sets,omitempty"`
	CardsBySet map[string]map[string]domain.Card `json:"cardsBySet,omitempty"`
}

type setsFileData struct {
	Sets map[string]domain.Set `json:"sets"`
}

type cardsFileData struct {
	CardsBySet map[string]map[string]domain.Card `json:"cardsBySet"`
}

type collectionFileData struct {
	Collection map[string]domain.CollectionEntry `json:"collection"`
}

func Load(mainPath, setsPath, cardsPath, collectionPath string) (*Store, error) {
	db := NewDB()

	mainData, mainExists, err := loadMain(mainPath)
	if err != nil {
		return nil, err
	}
	if mainExists {
		if mainData.Meta.SchemaVersion != SchemaVersion {
			raw, readErr := os.ReadFile(mainPath)
			if readErr != nil {
				return nil, fmt.Errorf("read legacy database: %w", readErr)
			}
			_ = backupLegacyDB(mainPath, raw, mainData.Meta.SchemaVersion)
			return &Store{
				mainPath:       mainPath,
				setsPath:       setsPath,
				cardsPath:      cardsPath,
				collectionPath: collectionPath,
				db:             NewDB(),
			}, nil
		}
		db.Meta = mainData.Meta
		db.SyncState = mainData.SyncState
		if mainData.Collection != nil {
			db.Collection = mainData.Collection
		}
		if mainData.Sets != nil {
			db.Sets = mainData.Sets
		}
		if mainData.CardsBySet != nil {
			db.CardsBySet = mainData.CardsBySet
		}
	}

	sets, setsExists, err := loadSets(setsPath)
	if err != nil {
		return nil, err
	}
	if setsExists {
		db.Sets = sets
	}

	cards, cardsExists, err := loadCards(cardsPath)
	if err != nil {
		return nil, err
	}
	if cardsExists {
		db.CardsBySet = cards
	}
	collection, collectionExists, err := loadCollection(collectionPath)
	if err != nil {
		return nil, err
	}
	if collectionExists {
		db.Collection = collection
	}

	db.ensureMaps()
	return &Store{
		mainPath:       mainPath,
		setsPath:       setsPath,
		cardsPath:      cardsPath,
		collectionPath: collectionPath,
		db:             db,
	}, nil
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
	mainData, err := marshalIndentNoEscapeHTML(mainFileData{
		Meta:      s.db.Meta,
		SyncState: s.db.SyncState,
	})
	if err != nil {
		return fmt.Errorf("encode database: %w", err)
	}
	setsData, err := marshalIndentNoEscapeHTML(setsFileData{
		Sets: s.db.Sets,
	})
	if err != nil {
		return fmt.Errorf("encode sets database: %w", err)
	}
	cardsData, err := marshalIndentNoEscapeHTML(cardsFileData{
		CardsBySet: s.db.CardsBySet,
	})
	if err != nil {
		return fmt.Errorf("encode cards database: %w", err)
	}
	collectionData, err := marshalIndentNoEscapeHTML(collectionFileData{
		Collection: s.db.Collection,
	})
	if err != nil {
		return fmt.Errorf("encode collection database: %w", err)
	}
	if err := WriteFileAtomically(s.setsPath, setsData, 0o600); err != nil {
		return fmt.Errorf("write sets database: %w", err)
	}
	if err := WriteFileAtomically(s.cardsPath, cardsData, 0o600); err != nil {
		return fmt.Errorf("write cards database: %w", err)
	}
	if err := WriteFileAtomically(s.collectionPath, collectionData, 0o600); err != nil {
		return fmt.Errorf("write collection database: %w", err)
	}
	if err := WriteFileAtomically(s.mainPath, mainData, 0o600); err != nil {
		return fmt.Errorf("write main database: %w", err)
	}
	return nil
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

func loadMain(path string) (mainFileData, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return mainFileData{}, false, nil
		}
		return mainFileData{}, false, fmt.Errorf("read main database: %w", err)
	}
	var parsed mainFileData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return mainFileData{}, false, fmt.Errorf("decode main database: %w", err)
	}
	return parsed, true, nil
}

func loadSets(path string) (map[string]domain.Set, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read sets database: %w", err)
	}
	var parsed setsFileData
	if err := json.Unmarshal(data, &parsed); err == nil && parsed.Sets != nil {
		return parsed.Sets, true, nil
	}
	var direct map[string]domain.Set
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, false, fmt.Errorf("decode sets database: %w", err)
	}
	return direct, true, nil
}

func loadCards(path string) (map[string]map[string]domain.Card, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read cards database: %w", err)
	}
	var parsed cardsFileData
	if err := json.Unmarshal(data, &parsed); err == nil && parsed.CardsBySet != nil {
		return parsed.CardsBySet, true, nil
	}
	var direct map[string]map[string]domain.Card
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, false, fmt.Errorf("decode cards database: %w", err)
	}
	return direct, true, nil
}

func loadCollection(path string) (map[string]domain.CollectionEntry, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read collection database: %w", err)
	}
	var parsed collectionFileData
	if err := json.Unmarshal(data, &parsed); err == nil && parsed.Collection != nil {
		return parsed.Collection, true, nil
	}
	var direct map[string]domain.CollectionEntry
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, false, fmt.Errorf("decode collection database: %w", err)
	}
	return direct, true, nil
}
