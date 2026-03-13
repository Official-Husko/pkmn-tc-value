package bootstrap

import (
	"context"
	"net/http"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/catalog/tcgdex"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	trackerpricing "github.com/Official-Husko/pkmn-tc-value/internal/pricing/pokemonpricetracker"
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
	PriceBridge *trackerpricing.Resolver
	Pricing     pricing.Provider
	Tracker     *trackerpricing.Client
	KeyRing     *trackerpricing.KeyRing
	ImageCache  *images.Cache
	Images      *images.Downloader
	Renderer    images.Renderer

	APIKeys    *repository.APIKeysRepo
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
	catalogProvider := tcgdex.New(httpClient, responseLogger, cfg.APIKeys)
	apiKeysRepo := repository.NewAPIKeysRepo(db)
	keyRing := trackerpricing.NewKeyRing(cfg.APIKeys, cfg.APIKeyDailyLimit, apiKeysRepo)
	trackerClient := trackerpricing.NewClient(httpClient, keyRing, responseLogger)
	priceBridge := trackerpricing.NewResolver(trackerClient)
	priceProvider := trackerpricing.NewProvider(trackerClient, priceBridge)
	cache := images.NewCache(paths.ImageDir)
	downloader := images.NewDownloader(httpClient, cache, cfg.APIKeys, cfg.UserAgent, cfg.BackupImageSource, cfg.Debug, paths.DebugLog)

	return &Container{
		Config:      cfg,
		Paths:       paths,
		Store:       db,
		Catalog:     catalogProvider,
		PriceBridge: priceBridge,
		Pricing:     priceProvider,
		Tracker:     trackerClient,
		KeyRing:     keyRing,
		ImageCache:  cache,
		Images:      downloader,
		Renderer:    images.NewRenderer(),
		APIKeys:     apiKeysRepo,
		Sets:        repository.NewSetsRepo(db),
		Cards:       repository.NewCardsRepo(db),
		Collection:  repository.NewCollectionRepo(db),
		SyncState:   repository.NewSyncStateRepo(db),
		StartupSync: syncer.NewStartupService(db, catalogProvider),
		SetSync:     syncer.NewSetSyncService(db, catalogProvider, priceProvider, downloader, priceBridge),
		CardRefresh: syncer.NewCardRefreshService(db, priceProvider, downloader),
	}
}

func (c *Container) ValidateAPIKeys(ctx context.Context) (trackerpricing.ValidationSummary, error) {
	if c == nil || c.Tracker == nil {
		return trackerpricing.ValidationSummary{}, nil
	}
	return c.Tracker.ValidateKeys(ctx, c.Config.UserAgent)
}
