package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type Paths struct {
	ConfigDir  string
	CacheDir   string
	ConfigFile string
	DBFile     string
	LockFile   string
	ImageDir   string
}

func ResolvePaths() (Paths, error) {
	root, err := os.Getwd()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return Paths{
		ConfigDir:  root,
		CacheDir:   root,
		ConfigFile: filepath.Join(root, "config.json"),
		DBFile:     filepath.Join(root, "db.json"),
		LockFile:   filepath.Join(root, "db.lock"),
		ImageDir:   filepath.Join(root, "cards"),
	}, nil
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
