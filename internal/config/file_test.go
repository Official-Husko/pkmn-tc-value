package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathsUsesDataDirectory(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	dataDir := filepath.Join(tmp, "data")
	if paths.ConfigDir != dataDir {
		t.Fatalf("expected ConfigDir=%q, got %q", dataDir, paths.ConfigDir)
	}
	if paths.DBFile != filepath.Join(dataDir, "database.db") {
		t.Fatalf("unexpected DBFile: %q", paths.DBFile)
	}
	if paths.SetsDBFile != filepath.Join(dataDir, "sets.db") {
		t.Fatalf("unexpected SetsDBFile: %q", paths.SetsDBFile)
	}
	if paths.CardsDBFile != filepath.Join(dataDir, "cards.db") {
		t.Fatalf("unexpected CardsDBFile: %q", paths.CardsDBFile)
	}
	if paths.CollectionDBFile != filepath.Join(dataDir, "collection.db") {
		t.Fatalf("unexpected CollectionDBFile: %q", paths.CollectionDBFile)
	}
}

func TestMigrateLegacyLayoutMovesLegacyFiles(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	legacyCardsDir := filepath.Join(tmp, "cards")
	if err := os.MkdirAll(legacyCardsDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy cards: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "db.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sets.db"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy sets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "cards.db"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy cards: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "collection.db"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy collection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "debug.log"), []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy debug: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyCardsDir, "sample.png"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write legacy card image: %v", err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if err := MigrateLegacyLayout(paths); err != nil {
		t.Fatalf("migrate legacy layout: %v", err)
	}

	required := []string{
		paths.ConfigFile,
		paths.DBFile,
		paths.SetsDBFile,
		paths.CardsDBFile,
		paths.CollectionDBFile,
		paths.DebugLog,
		filepath.Join(paths.ImageDir, "sample.png"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migrated path %q: %v", p, err)
		}
	}
}

func TestMigrateLegacyLayoutMovesDataDBDotDB(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	legacyDataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(legacyDataDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDataDir, "db.db"), []byte(`{"meta":{"schemaVersion":2}}`), 0o600); err != nil {
		t.Fatalf("write legacy data/db.db: %v", err)
	}

	paths, err := ResolvePaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if err := MigrateLegacyLayout(paths); err != nil {
		t.Fatalf("migrate legacy layout: %v", err)
	}

	if _, err := os.Stat(paths.DBFile); err != nil {
		t.Fatalf("expected migrated database file %q: %v", paths.DBFile, err)
	}
	if _, err := os.Stat(filepath.Join(legacyDataDir, "db.db")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy data/db.db to be moved, stat err=%v", err)
	}
}
