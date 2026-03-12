package config

type Config struct {
	StartupSyncEnabled            bool     `json:"startupSyncEnabled"`
	Debug                         bool     `json:"debug"`
	APIKeys                       []string `json:"apiKeys,omitempty"`
	Hotkeys                       map[string]string `json:"hotkeys,omitempty"`
	APIKeyDailyLimit              int      `json:"apiKeyDailyLimit"`
	CardRefreshTTLHours           int      `json:"cardRefreshTTLHours"`
	ImagePreviewsEnabled          bool     `json:"imagePreviewsEnabled"`
	ImageCaching                  bool     `json:"imageCaching"`
	PrefetchCardMetadataOnStartup bool     `json:"prefetchCardMetadataOnStartup"`
	DownloadAllImagesOnStartup    bool     `json:"downloadAllImagesOnStartup"`
	ImageDownloadWorkers          int      `json:"imageDownloadWorkers"`
	BackupImageSource             bool     `json:"backup_image_source"`
	SyncCardDetails               bool     `json:"syncCardDetails"`
	ColorsEnabled                 bool     `json:"colorsEnabled"`
	RequestDelayMs                int      `json:"requestDelayMs"`
	RateLimitCooldownSeconds      int      `json:"rateLimitCooldownSeconds"`
	SaveSearchedCardsDefault      bool     `json:"saveSearchedCardsDefault"`
	LastViewedSetOnTop            bool     `json:"lastViewedSetOnTop"`
	UserAgent                     string   `json:"userAgent"`
}
