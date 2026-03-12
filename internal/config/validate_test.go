package config

import "testing"

func TestValidate(t *testing.T) {
	cfg := Default()
	if cfg.PrefetchCardMetadataOnStartup {
		t.Fatal("prefetch card metadata on startup should default to false")
	}
	if cfg.DownloadAllImagesOnStartup {
		t.Fatal("download all images on startup should default to false")
	}
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
	cfg.ImageDownloadWorkers = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected image download worker validation error")
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

	cfg = Default()
	cfg.APIKeyDailyLimit = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected API key daily limit validation error")
	}

	cfg = Default()
	cfg.APIKeys = []string{"abc", "  "}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected blank API key validation error")
	}

	cfg = Default()
	cfg.APIKeys = []string{"dup", "dup"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate API key validation error")
	}

	cfg = Default()
	cfg.Hotkeys["move_up"] = " "
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected blank hotkey validation error")
	}

	cfg = Default()
	cfg.Hotkeys["move_up"] = "j"
	cfg.Hotkeys["move_down"] = "j"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate hotkey validation error")
	}
}
