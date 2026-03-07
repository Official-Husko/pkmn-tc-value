package config

import (
	"errors"
	"strings"
)

func (c Config) Validate() error {
	switch {
	case c.CardRefreshTTLHours < 1 || c.CardRefreshTTLHours > 168:
		return errors.New("card refresh TTL must be between 1 and 168 hours")
	case c.RequestDelayMs < 250 || c.RequestDelayMs > 10000:
		return errors.New("request delay must be between 250 and 10000 ms")
	case c.RateLimitCooldownSeconds < 1 || c.RateLimitCooldownSeconds > 300:
		return errors.New("rate-limit cooldown must be between 1 and 300 seconds")
	case strings.TrimSpace(c.UserAgent) == "":
		return errors.New("user agent cannot be blank")
	default:
		return nil
	}
}
