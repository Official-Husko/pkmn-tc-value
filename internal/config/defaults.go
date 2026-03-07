package config

func Default() Config {
	return Config{
		StartupSyncEnabled:       true,
		CardRefreshTTLHours:      48,
		ImagePreviewsEnabled:     true,
		SaveCardImages:           true,
		SyncCardDetails:          false,
		ColorsEnabled:            true,
		RequestDelayMs:           1200,
		RateLimitCooldownSeconds: 30,
		SaveSearchedCardsDefault: true,
		UserAgent:                "pkmn-tc-value/1.0 (by Official Husko on GitHub)",
	}
}
