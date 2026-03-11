package app

import (
	"context"
	"os"

	"github.com/Official-Husko/pkmn-tc-value/internal/bootstrap"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

func Run(ctx context.Context) error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	if err := config.MigrateLegacyLayout(paths); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.CacheDir, 0o755); err != nil {
		return err
	}
	lock, err := store.AcquireLock(paths.LockFile)
	if err != nil {
		return err
	}
	defer lock.Release()

	cfg, err := config.LoadOrCreate(paths.ConfigFile)
	if err != nil {
		return err
	}
	if cfg.Debug {
		if err := os.MkdirAll(paths.LogsDir, 0o755); err != nil {
			return err
		}
	}
	db, err := store.Load(paths.DBFile, paths.SetsDBFile, paths.CardsDBFile, paths.CollectionDBFile)
	if err != nil {
		return err
	}
	if err := db.Update(func(*store.DB) error { return nil }); err != nil {
		return err
	}
	container := bootstrap.New(cfg, paths, db)
	return New(container).Run(ctx)
}
