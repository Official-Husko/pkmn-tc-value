package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
	path := filepath.Join(dir, "db.json")
	legacy := []byte(`{
  "meta":{"schemaVersion":1,"createdAt":"2026-03-08T00:00:00Z","updatedAt":"2026-03-08T00:00:00Z"},
  "syncState":{"catalogProvider":"pokedata","priceProvider":"pokedata"},
  "sets":{"legacy":{"id":"legacy","name":"Old","total":1}},
  "cardsBySet":{"legacy":{"legacy-1":{"id":"legacy-1","setId":"legacy","name":"Old Card","number":"1"}}},
  "collection":{}
}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	s, err := Load(path)
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
