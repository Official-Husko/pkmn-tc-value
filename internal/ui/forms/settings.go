package forms

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
)

const (
	settingStartupSync      = "startup_sync"
	settingCardRefreshTTL   = "card_refresh_ttl"
	settingImagePreviews    = "image_previews"
	settingSaveCardImages   = "save_card_images"
	settingSyncCardDetails  = "sync_card_details"
	settingColors           = "colors"
	settingRequestDelay     = "request_delay"
	settingRateLimitCooloff = "rate_limit_cooldown"
	settingSaveSearched     = "save_searched"
	settingUserAgent        = "user_agent"
	settingSaveAndBack      = "save_back"
	settingBackNoSave       = "back_no_save"
)

func SettingsForm(cfg config.Config, theme *huh.Theme) (config.Config, error) {
	next := cfg
	for {
		choice, err := pickSetting(next, theme)
		if err != nil {
			return cfg, err
		}
		switch choice {
		case settingStartupSync:
			value, err := editBoolSetting(
				"Startup Sync",
				"Fetch the latest set catalog when the app starts.",
				next.StartupSyncEnabled,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.StartupSyncEnabled = value
		case settingCardRefreshTTL:
			value, err := editIntSetting(
				"Card Refresh TTL (hours)",
				"How old card price data can be before auto-refresh starts when opening a card.",
				next.CardRefreshTTLHours,
				1,
				168,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.CardRefreshTTLHours = value
		case settingImagePreviews:
			value, err := editBoolSetting(
				"Image Previews",
				"Render card images in the detail view when terminal support is available.",
				next.ImagePreviewsEnabled,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.ImagePreviewsEnabled = value
		case settingSaveCardImages:
			value, err := editBoolSetting(
				"Save Card Images",
				"When syncing a set database, download card images and store them in the local cache.",
				next.SaveCardImages,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.SaveCardImages = value
		case settingSyncCardDetails:
			value, err := editBoolSetting(
				"Sync Card Details (prices)",
				"When syncing a set database, also fetch per-card detail stats and prices.",
				next.SyncCardDetails,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.SyncCardDetails = value
		case settingColors:
			value, err := editBoolSetting(
				"Colors",
				"Enable themed colors in menus, labels, and status messages.",
				next.ColorsEnabled,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.ColorsEnabled = value
		case settingRequestDelay:
			value, err := editIntSetting(
				"Request Delay (ms)",
				"Delay between scraper requests to avoid hammering the provider.",
				next.RequestDelayMs,
				250,
				10000,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.RequestDelayMs = value
		case settingRateLimitCooloff:
			value, err := editIntSetting(
				"Rate-limit Cooldown (seconds)",
				"How long to wait before retrying after a 429 Too Many Requests response.",
				next.RateLimitCooldownSeconds,
				1,
				300,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.RateLimitCooldownSeconds = value
		case settingSaveSearched:
			value, err := editBoolSetting(
				"Save Searched Cards by Default",
				"When true, the default selected action in card detail is Add to collection.",
				next.SaveSearchedCardsDefault,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.SaveSearchedCardsDefault = value
		case settingUserAgent:
			value, err := editTextSetting(
				"HTTP User Agent",
				"User-Agent header sent to remote APIs and scraper requests.",
				next.UserAgent,
				false,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.UserAgent = value
		case settingSaveAndBack:
			return next, next.Validate()
		case settingBackNoSave:
			return cfg, nil
		}
	}
}

func pickSetting(cfg config.Config, theme *huh.Theme) (string, error) {
	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Settings").
				Description("Select one option to edit. Save when you're done.").
				Height(14).
				Options(
					huh.NewOption(fmt.Sprintf("Startup sync: %s", onOff(cfg.StartupSyncEnabled)), settingStartupSync),
					huh.NewOption(fmt.Sprintf("Card refresh TTL: %d hours", cfg.CardRefreshTTLHours), settingCardRefreshTTL),
					huh.NewOption(fmt.Sprintf("Image previews: %s", onOff(cfg.ImagePreviewsEnabled)), settingImagePreviews),
					huh.NewOption(fmt.Sprintf("Save card images: %s", onOff(cfg.SaveCardImages)), settingSaveCardImages),
					huh.NewOption(fmt.Sprintf("Sync card details: %s", onOff(cfg.SyncCardDetails)), settingSyncCardDetails),
					huh.NewOption(fmt.Sprintf("Colors: %s", onOff(cfg.ColorsEnabled)), settingColors),
					huh.NewOption(fmt.Sprintf("Request delay: %d ms", cfg.RequestDelayMs), settingRequestDelay),
					huh.NewOption(fmt.Sprintf("Rate-limit cooldown: %d sec", cfg.RateLimitCooldownSeconds), settingRateLimitCooloff),
					huh.NewOption(fmt.Sprintf("Save searched cards by default: %s", onOff(cfg.SaveSearchedCardsDefault)), settingSaveSearched),
					huh.NewOption(fmt.Sprintf("HTTP user agent: %s", cfg.UserAgent), settingUserAgent),
					huh.NewOption("Save and Back", settingSaveAndBack),
					huh.NewOption("Back without saving", settingBackNoSave),
				).
				Value(&choice),
		),
	).WithTheme(theme)
	return choice, form.Run()
}

func editBoolSetting(title, description string, current bool, theme *huh.Theme) (bool, error) {
	value := current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&value),
		),
	).WithTheme(theme)
	return value, form.Run()
}

func editIntSetting(title, description string, current int, min int, max int, theme *huh.Theme) (int, error) {
	value := strconv.Itoa(current)
	for {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(title).
					Description(fmt.Sprintf("%s Allowed range: %d to %d.", description, min, max)).
					Value(&value),
			),
		).WithTheme(theme)
		if err := form.Run(); err != nil {
			return current, err
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			if err := showSettingsError("Invalid number. Enter digits only.", theme); err != nil {
				return current, err
			}
			continue
		}
		if parsed < min || parsed > max {
			msg := fmt.Sprintf("Value must be between %d and %d.", min, max)
			if err := showSettingsError(msg, theme); err != nil {
				return current, err
			}
			continue
		}
		return parsed, nil
	}
}

func editTextSetting(title, description string, current string, allowBlank bool, theme *huh.Theme) (string, error) {
	value := current
	for {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(title).
					Description(description).
					Value(&value),
			),
		).WithTheme(theme)
		if err := form.Run(); err != nil {
			return current, err
		}
		value = strings.TrimSpace(value)
		if value == "" && !allowBlank {
			if err := showSettingsError("This value cannot be blank.", theme); err != nil {
				return current, err
			}
			continue
		}
		return value, nil
	}
}

func showSettingsError(message string, theme *huh.Theme) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Settings Error").
				Description(message).
				Next(true).
				NextLabel("Back"),
		),
	).WithTheme(theme).Run()
}

func onOff(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}
