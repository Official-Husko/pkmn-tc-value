package config

type Config struct {
	StartupSyncEnabled       bool   `json:"startupSyncEnabled"`
	Debug                    bool   `json:"debug"`
	CardRefreshTTLHours      int    `json:"cardRefreshTTLHours"`
	ImagePreviewsEnabled     bool   `json:"imagePreviewsEnabled"`
	ImageCaching             bool   `json:"imageCaching"`
	BackupImageSource        bool   `json:"backup_image_source"`
	SyncCardDetails          bool   `json:"syncCardDetails"`
	ColorsEnabled            bool   `json:"colorsEnabled"`
	RequestDelayMs           int    `json:"requestDelayMs"`
	RateLimitCooldownSeconds int    `json:"rateLimitCooldownSeconds"`
	SaveSearchedCardsDefault bool   `json:"saveSearchedCardsDefault"`
	UserAgent                string `json:"userAgent"`
}
