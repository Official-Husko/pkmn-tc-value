package config

type Config struct {
	StartupSyncEnabled       bool   `json:"startupSyncEnabled"`
	CardRefreshTTLHours      int    `json:"cardRefreshTTLHours"`
	ImagePreviewsEnabled     bool   `json:"imagePreviewsEnabled"`
	SaveCardImages           bool   `json:"saveCardImages"`
	SyncCardDetails          bool   `json:"syncCardDetails"`
	ColorsEnabled            bool   `json:"colorsEnabled"`
	RequestDelayMs           int    `json:"requestDelayMs"`
	RateLimitCooldownSeconds int    `json:"rateLimitCooldownSeconds"`
	SaveSearchedCardsDefault bool   `json:"saveSearchedCardsDefault"`
	UserAgent                string `json:"userAgent"`
}
