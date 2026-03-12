package config

import (
	"errors"
	"slices"
	"strings"
)

func (c Config) Validate() error {
	switch {
	case c.APIKeyDailyLimit < 1 || c.APIKeyDailyLimit > 100000:
		return errors.New("API key daily limit must be between 1 and 100000")
	case c.CardRefreshTTLHours < 1 || c.CardRefreshTTLHours > 168:
		return errors.New("card refresh TTL must be between 1 and 168 hours")
	case c.ImageDownloadWorkers < 1 || c.ImageDownloadWorkers > 32:
		return errors.New("image download workers must be between 1 and 32")
	case c.RequestDelayMs < 250 || c.RequestDelayMs > 10000:
		return errors.New("request delay must be between 250 and 10000 ms")
	case c.RateLimitCooldownSeconds < 1 || c.RateLimitCooldownSeconds > 300:
		return errors.New("rate-limit cooldown must be between 1 and 300 seconds")
	case strings.TrimSpace(c.UserAgent) == "":
		return errors.New("user agent cannot be blank")
	default:
		for _, key := range c.APIKeys {
			if strings.TrimSpace(key) == "" {
				return errors.New("API keys cannot include blank values")
			}
		}
		if hasDuplicateAPIKeys(c.APIKeys) {
			return errors.New("API keys must be unique")
		}
		return nil
	}
}

func hasDuplicateAPIKeys(keys []string) bool {
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		normalized = append(normalized, strings.TrimSpace(key))
	}
	slices.Sort(normalized)
	for i := 1; i < len(normalized); i++ {
		if normalized[i-1] == normalized[i] {
			return true
		}
	}
	return false
}
