package bootstrap

import (
	"net/http"
	"time"

	bridgepokedata "github.com/Official-Husko/pkmn-tc-value/internal/bridge/pokedata"
	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/catalog/tcgdex"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	pricedata "github.com/Official-Husko/pkmn-tc-value/internal/pricing/pokedata"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
	"github.com/Official-Husko/pkmn-tc-value/internal/repository"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
)

type Container struct {
	Config      config.Config
	Paths       config.Paths
	Store       *store.Store
	Catalog     catalog.Provider
	PriceBridge *bridgepokedata.Resolver
	Pricing     pricing.Provider
	ImageCache  *images.Cache
	Images      *images.Downloader
	Renderer    images.Renderer

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
	responseLogger := providerslog.New(cfg.Debug, paths.LogsDir)
	catalogProvider := tcgdex.New(httpClient, responseLogger)
	priceBridge := bridgepokedata.NewResolver(httpClient, time.Duration(cfg.RateLimitCooldownSeconds)*time.Second, responseLogger)
	priceProvider := pricedata.New(httpClient, priceBridge, responseLogger)
	cache := images.NewCache(paths.ImageDir)
	downloader := images.NewDownloader(httpClient, cache, cfg.BackupImageSource, cfg.Debug, paths.DebugLog)

	return &Container{
		Config:      cfg,
		Paths:       paths,
		Store:       db,
		Catalog:     catalogProvider,
		PriceBridge: priceBridge,
		Pricing:     priceProvider,
		ImageCache:  cache,
		Images:      downloader,
		Renderer:    images.NewRenderer(),
		Sets:        repository.NewSetsRepo(db),
		Cards:       repository.NewCardsRepo(db),
		Collection:  repository.NewCollectionRepo(db),
		SyncState:   repository.NewSyncStateRepo(db),
		StartupSync: syncer.NewStartupService(db, catalogProvider),
		SetSync:     syncer.NewSetSyncService(db, catalogProvider, priceProvider, downloader, priceBridge),
		CardRefresh: syncer.NewCardRefreshService(db, priceProvider, downloader),
	}
}
