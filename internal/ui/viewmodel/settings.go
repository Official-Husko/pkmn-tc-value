package viewmodel

import (
	"fmt"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
)

type SettingsState struct {
	StartupSyncEnabled bool
	CardRefreshTTL     string
	ImagePreviews      bool
	ColorsEnabled      bool
	RequestDelay       string
	RateLimitCooldown  string
	SaveSearchedCards  bool
	UserAgent          string
}

func NewSettingsState(cfg config.Config) SettingsState {
	return SettingsState{
		StartupSyncEnabled: cfg.StartupSyncEnabled,
		CardRefreshTTL:     itoa(cfg.CardRefreshTTLHours),
		ImagePreviews:      cfg.ImagePreviewsEnabled,
		ColorsEnabled:      cfg.ColorsEnabled,
		RequestDelay:       itoa(cfg.RequestDelayMs),
		RateLimitCooldown:  itoa(cfg.RateLimitCooldownSeconds),
		SaveSearchedCards:  cfg.SaveSearchedCardsDefault,
		UserAgent:          cfg.UserAgent,
	}
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
