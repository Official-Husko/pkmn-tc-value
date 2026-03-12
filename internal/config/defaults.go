package config

func DefaultHotkeys() map[string]string {
	return map[string]string{
		"quit":        "ctrl+c",
		"back":        "esc",
		"confirm":     "enter",
		"filter":      "/",
		"set_jump_id": "i",
		"move_up":     "k",
		"move_down":   "j",
		"page_up":     "pgup",
		"page_down":   "pgdown",
		"go_top":      "home",
		"go_bottom":   "end",
		"card_add":    "a",
		"card_close":  "c",
		"card_left":   "h",
		"card_right":  "l",
		"main_browse": "b",
		"main_settings": "s",
		"main_quit":   "q",
	}
}

func Default() Config {
	return Config{
		StartupSyncEnabled:            true,
		Debug:                         false,
		APIKeys:                       nil,
		Hotkeys:                       DefaultHotkeys(),
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
