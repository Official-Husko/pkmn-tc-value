package bootstrap

import (
	"net/http"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/catalog/pokedata"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	pricedata "github.com/Official-Husko/pkmn-tc-value/internal/pricing/pokedata"
	"github.com/Official-Husko/pkmn-tc-value/internal/repository"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
)

type Container struct {
	Config     config.Config
	Paths      config.Paths
	Store      *store.Store
	Catalog    catalog.Provider
	Pricing    pricing.Provider
	ImageCache *images.Cache
	Images     *images.Downloader
	Renderer   images.Renderer

	Sets       *repository.SetsRepo
	Cards      *repository.CardsRepo
	Collection *repository.CollectionRepo
	SyncState  *repository.SyncStateRepo

	StartupSync *syncer.StartupService
	SetSync     *syncer.SetSyncService
	CardRefresh *syncer.CardRefreshService
}

func New(cfg config.Config, paths config.Paths, db *store.Store) *Container {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	catalogProvider := pokedata.New(httpClient, time.Duration(cfg.RateLimitCooldownSeconds)*time.Second)
	priceProvider := pricedata.New(httpClient)
	cache := images.NewCache(paths.ImageDir)
	downloader := images.NewDownloader(httpClient, cache)

	return &Container{
		Config:      cfg,
		Paths:       paths,
		Store:       db,
		Catalog:     catalogProvider,
		Pricing:     priceProvider,
		ImageCache:  cache,
		Images:      downloader,
		Renderer:    images.NewRenderer(),
		Sets:        repository.NewSetsRepo(db),
		Cards:       repository.NewCardsRepo(db),
		Collection:  repository.NewCollectionRepo(db),
		SyncState:   repository.NewSyncStateRepo(db),
		StartupSync: syncer.NewStartupService(db, catalogProvider),
		SetSync:     syncer.NewSetSyncService(db, catalogProvider, priceProvider, downloader),
		CardRefresh: syncer.NewCardRefreshService(db, priceProvider, downloader),
	}
}
