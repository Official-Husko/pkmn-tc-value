package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type Paths struct {
	ConfigDir        string
	CacheDir         string
	ConfigFile       string
	DBFile           string
	SetsDBFile       string
	CardsDBFile      string
	CollectionDBFile string
	LockFile         string
	ImageDir         string
	DebugLog         string
}

func ResolvePaths() (Paths, error) {
	root, err := os.Getwd()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve working directory: %w", err)
	}
	dataDir := filepath.Join(root, "data")
	return Paths{
		ConfigDir:        dataDir,
		CacheDir:         dataDir,
		ConfigFile:       filepath.Join(dataDir, "config.json"),
		DBFile:           filepath.Join(dataDir, "database.db"),
		SetsDBFile:       filepath.Join(dataDir, "sets.db"),
		CardsDBFile:      filepath.Join(dataDir, "cards.db"),
		CollectionDBFile: filepath.Join(dataDir, "collection.db"),
		LockFile:         filepath.Join(dataDir, "db.lock"),
		ImageDir:         filepath.Join(dataDir, "cards"),
		DebugLog:         filepath.Join(dataDir, "debug.log"),
	}, nil
}

func MigrateLegacyLayout(paths Paths) error {
	root := filepath.Dir(paths.ConfigDir)
	legacyToNew := [][2]string{
		{filepath.Join(root, "config.json"), paths.ConfigFile},
		{filepath.Join(paths.ConfigDir, "db.db"), paths.DBFile},
		{filepath.Join(root, "db.db"), paths.DBFile},
		{filepath.Join(root, "database.db"), paths.DBFile},
		{filepath.Join(root, "db.json"), paths.DBFile},
		{filepath.Join(root, "sets.db"), paths.SetsDBFile},
		{filepath.Join(root, "cards.db"), paths.CardsDBFile},
		{filepath.Join(root, "collection.db"), paths.CollectionDBFile},
		{filepath.Join(root, "cards"), paths.ImageDir},
		{filepath.Join(root, "debug.log"), paths.DebugLog},
	}
	for _, pair := range legacyToNew {
		src := pair[0]
		dst := pair[1]
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat legacy path %q: %w", src, err)
		}
		if _, err := os.Stat(dst); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat destination path %q: %w", dst, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create destination directory for %q: %w", dst, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate %q to %q: %w", src, dst, err)
		}
	}
	return nil
}

func LoadOrCreate(path string) (Config, error) {
	cfg := Default()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Config{}, fmt.Errorf("create config dir: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := Save(path, cfg); err != nil {
				return Config{}, err
			}
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config file: %w", err)
	}
	var legacy struct {
		ImageCaching   *bool `json:"imageCaching"`
		SaveCardImages *bool `json:"saveCardImages"`
	}
	if err := json.Unmarshal(data, &legacy); err == nil {
		if legacy.ImageCaching == nil && legacy.SaveCardImages != nil {
			cfg.ImageCaching = *legacy.SaveCardImages
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	return store.WriteFileAtomically(path, data, 0o600)
}
