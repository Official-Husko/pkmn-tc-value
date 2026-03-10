package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestMarshalIndentNoEscapeHTML(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}
	out, err := marshalIndentNoEscapeHTML(sample{Name: "Scarlet & Violet"})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if bytes.Contains(out, []byte(`\u0026`)) {
		t.Fatalf("expected '&' to be preserved, got: %s", string(out))
	}
	if !bytes.Contains(out, []byte(`Scarlet & Violet`)) {
		t.Fatalf("expected readable '&' output, got: %s", string(out))
	}
}

func TestLoadResetsLegacySchema(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "db.json")
	setsPath := filepath.Join(dir, "sets.db")
	cardsPath := filepath.Join(dir, "cards.db")
	collectionPath := filepath.Join(dir, "collection.db")
	legacy := []byte(`{
  "meta":{"schemaVersion":1,"createdAt":"2026-03-08T00:00:00Z","updatedAt":"2026-03-08T00:00:00Z"},
  "syncState":{"catalogProvider":"pokedata","priceProvider":"pokedata"},
  "sets":{"legacy":{"id":"legacy","name":"Old","total":1}},
  "cardsBySet":{"legacy":{"legacy-1":{"id":"legacy-1","setId":"legacy","name":"Old Card","number":"1"}}},
  "collection":{}
}`)
	if err := os.WriteFile(mainPath, legacy, 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	s, err := Load(mainPath, setsPath, cardsPath, collectionPath)
	if err != nil {
		t.Fatalf("load should succeed: %v", err)
	}

	if s.db.Meta.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema %d, got %d", SchemaVersion, s.db.Meta.SchemaVersion)
	}
	if len(s.db.Sets) != 0 {
		t.Fatalf("expected reset db with no sets, got %d", len(s.db.Sets))
	}

	matches, err := filepath.Glob(filepath.Join(dir, "db.json.schema-1.*.bak"))
	if err != nil {
		t.Fatalf("glob backup files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected legacy backup file to be created")
	}
}

func TestSplitDatabaseFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "db.json")
	setsPath := filepath.Join(dir, "sets.db")
	cardsPath := filepath.Join(dir, "cards.db")
	collectionPath := filepath.Join(dir, "collection.db")

	s, err := Load(mainPath, setsPath, cardsPath, collectionPath)
	if err != nil {
		t.Fatalf("load empty split db: %v", err)
	}
	err = s.Update(func(db *DB) error {
		db.Sets["sv4a"] = domain.Set{
			ID:    "sv4a",
			Name:  "Shiny Treasure ex",
			Total: 190,
		}
		db.CardsBySet["sv4a"] = map[string]domain.Card{
			"001": {
				ID:     "sv4a-001",
				SetID:  "sv4a",
				Name:   "Bulbasaur",
				Number: "001",
			},
		}
		now := time.Now().UTC()
		db.Collection["sv4a-001"] = domain.CollectionEntry{
			CardID:    "sv4a-001",
			Quantity:  2,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update split db: %v", err)
	}

	mainRaw, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main db file: %v", err)
	}
	if bytes.Contains(mainRaw, []byte(`"sets"`)) {
		t.Fatalf("main db should not contain sets payload: %s", string(mainRaw))
	}
	if bytes.Contains(mainRaw, []byte(`"cardsBySet"`)) {
		t.Fatalf("main db should not contain cards payload: %s", string(mainRaw))
	}
	if bytes.Contains(mainRaw, []byte(`"collection"`)) {
		t.Fatalf("main db should not contain collection payload: %s", string(mainRaw))
	}

	setsRaw, err := os.ReadFile(setsPath)
	if err != nil {
		t.Fatalf("read sets db file: %v", err)
	}
	if !bytes.Contains(setsRaw, []byte(`"sets"`)) {
		t.Fatalf("sets file missing sets payload: %s", string(setsRaw))
	}

	cardsRaw, err := os.ReadFile(cardsPath)
	if err != nil {
		t.Fatalf("read cards db file: %v", err)
	}
	if !bytes.Contains(cardsRaw, []byte(`"cardsBySet"`)) {
		t.Fatalf("cards file missing cardsBySet payload: %s", string(cardsRaw))
	}

	collectionRaw, err := os.ReadFile(collectionPath)
	if err != nil {
		t.Fatalf("read collection db file: %v", err)
	}
	if !bytes.Contains(collectionRaw, []byte(`"collection"`)) {
		t.Fatalf("collection file missing collection payload: %s", string(collectionRaw))
	}

	reloaded, err := Load(mainPath, setsPath, cardsPath, collectionPath)
	if err != nil {
		t.Fatalf("reload split db: %v", err)
	}
	if len(reloaded.db.Sets) != 1 {
		t.Fatalf("expected 1 set after reload, got %d", len(reloaded.db.Sets))
	}
	if len(reloaded.db.CardsBySet["sv4a"]) != 1 {
		t.Fatalf("expected 1 card after reload, got %d", len(reloaded.db.CardsBySet["sv4a"]))
	}
	if got := reloaded.db.Collection["sv4a-001"].Quantity; got != 2 {
		t.Fatalf("expected collection quantity 2 after reload, got %d", got)
	}
}

func TestLoadUsesLegacyMainCollectionWhenCollectionFileMissing(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "database.db")
	setsPath := filepath.Join(dir, "sets.db")
	cardsPath := filepath.Join(dir, "cards.db")
	collectionPath := filepath.Join(dir, "collection.db")
	legacyMain := []byte(`{
  "meta":{"schemaVersion":2,"createdAt":"2026-03-08T00:00:00Z","updatedAt":"2026-03-08T00:00:00Z"},
  "syncState":{"catalogProvider":"tcgdex","priceProvider":"pokedata"},
  "collection":{
    "sv4a-001":{"cardId":"sv4a-001","quantity":3,"createdAt":"2026-03-08T00:00:00Z","updatedAt":"2026-03-08T00:00:00Z"}
  }
}`)
	if err := os.WriteFile(mainPath, legacyMain, 0o600); err != nil {
		t.Fatalf("write legacy-style main db: %v", err)
	}

	s, err := Load(mainPath, setsPath, cardsPath, collectionPath)
	if err != nil {
		t.Fatalf("load db: %v", err)
	}
	if got := s.db.Collection["sv4a-001"].Quantity; got != 3 {
		t.Fatalf("expected legacy collection fallback quantity 3, got %d", got)
	}
}
