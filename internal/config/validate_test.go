package config

import "testing"

func TestValidate(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}

	cfg.CardRefreshTTLHours = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected ttl validation error")
	}

	cfg = Default()
	cfg.RequestDelayMs = 100
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected request delay validation error")
	}

	cfg = Default()
	cfg.UserAgent = "   "
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected user agent validation error")
	}

	cfg = Default()
	cfg.RateLimitCooldownSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected cooldown validation error")
	}
}
