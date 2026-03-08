package config

func Default() Config {
	return Config{
		StartupSyncEnabled:       true,
		Debug:                    false,
		CardRefreshTTLHours:      48,
		ImagePreviewsEnabled:     true,
		ImageCaching:             true,
		ImageDownloadWorkers:     6,
		BackupImageSource:        false,
		SyncCardDetails:          false,
		ColorsEnabled:            true,
		RequestDelayMs:           1200,
		RateLimitCooldownSeconds: 30,
		SaveSearchedCardsDefault: true,
		UserAgent:                "pkmn-tc-value/1.0 (by Official Husko on GitHub)",
	}
}
