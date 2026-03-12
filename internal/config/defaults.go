package config

func Default() Config {
	return Config{
		StartupSyncEnabled:            true,
		Debug:                         false,
		APIKeys:                       nil,
		APIKeyDailyLimit:              100,
		CardRefreshTTLHours:           48,
		ImagePreviewsEnabled:          true,
		ImageCaching:                  true,
		PrefetchCardMetadataOnStartup: false,
		DownloadAllImagesOnStartup:    false,
		ImageDownloadWorkers:          6,
		BackupImageSource:             false,
		SyncCardDetails:               false,
		ColorsEnabled:                 true,
		RequestDelayMs:                1200,
		RateLimitCooldownSeconds:      30,
		SaveSearchedCardsDefault:      true,
		LastViewedSetOnTop:            true,
		UserAgent:                     "pkmn-tc-value/1.0 (by Official Husko on GitHub)",
	}
}
