package forms

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	_ "golang.org/x/image/webp"
)

const (
	imageCompatTestURL      = "https://pokemoncardimages.pokedata.io/images/Shiny+Treasure+ex/349.webp"
	settingStartupSync      = "startup_sync"
	settingCardRefreshTTL   = "card_refresh_ttl"
	settingImagePreviews    = "image_previews"
	settingImageCompatTest  = "image_compat_test"
	settingImageCaching     = "image_caching"
	settingStartupMetadata  = "startup_prefetch_metadata"
	settingStartupAllImages = "startup_all_images"
	settingSyncCardDetails  = "sync_card_details"
	settingColors           = "colors"
	settingRequestDelay     = "request_delay"
	settingRateLimitCooloff = "rate_limit_cooldown"
	settingSaveSearched     = "save_searched"
	settingUserAgent        = "user_agent"
	settingSaveAndBack      = "save_back"
	settingBackNoSave       = "back_no_save"
)

func SettingsForm(cfg config.Config, renderer images.Renderer, theme *huh.Theme) (config.Config, error) {
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
		case settingImageCompatTest:
			visible, err := runImageCompatibilityTest(renderer, theme)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.ImagePreviewsEnabled = visible
			next.ImageCaching = visible
		case settingImageCaching:
			value, err := editBoolSetting(
				"Image Caching",
				"Store converted PNG card images in the local cache for faster future views.",
				next.ImageCaching,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.ImageCaching = value
		case settingStartupMetadata:
			value, err := editBoolSetting(
				"Prefetch Card Metadata on Startup",
				"When enabled, startup sync loads full card metadata for all sets (without downloading images).",
				next.PrefetchCardMetadataOnStartup,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.PrefetchCardMetadataOnStartup = value
		case settingStartupAllImages:
			value, err := editBoolSetting(
				"Download All Images on Startup",
				"When enabled, startup sync prefetches images for all sets. This can take a while.",
				next.DownloadAllImagesOnStartup,
				theme,
			)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return cfg, err
			}
			next.DownloadAllImagesOnStartup = value
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
					huh.NewOption("Test image compatibility", settingImageCompatTest),
					huh.NewOption(fmt.Sprintf("Image caching: %s", onOff(cfg.ImageCaching)), settingImageCaching),
					huh.NewOption(fmt.Sprintf("Prefetch card metadata on startup: %s", onOff(cfg.PrefetchCardMetadataOnStartup)), settingStartupMetadata),
					huh.NewOption(fmt.Sprintf("Download all images on startup: %s", onOff(cfg.DownloadAllImagesOnStartup)), settingStartupAllImages),
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

func runImageCompatibilityTest(renderer images.Renderer, theme *huh.Theme) (bool, error) {
	if renderer == nil {
		return false, nil
	}

	path, err := downloadCompatibilityImage(imageCompatTestURL)
	if err != nil {
		return false, err
	}
	defer os.Remove(path)

	rendered, renderErr := renderer.Render(path, 32, 12)
	if rendered == "" {
		rendered = "[image unavailable]"
	}
	description := "If you can see the sample image below, choose Visible.\n\n" + rendered
	if renderErr != nil {
		description += "\n\nRenderer error: " + renderErr.Error()
	}
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Image Compatibility Test").
				Description(description).
				Next(true).
				NextLabel("Continue"),
		),
	).WithTheme(theme).Run(); err != nil {
		return false, err
	}

	var result string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Was the image visible?").
				Description("This will automatically configure image preview and image download settings.").
				Options(
					huh.NewOption("Visible", "visible"),
					huh.NewOption("Not visible", "not_visible"),
				).
				Value(&result),
		),
	).WithTheme(theme)
	if err := form.Run(); err != nil {
		return false, err
	}
	return result == "visible", nil
}

func downloadCompatibilityImage(imageURL string) (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download test image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("download test image failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read test image: %w", err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode test image: %w", err)
	}
	file, err := os.CreateTemp("", "pkmn-image-test-*.png")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := png.Encode(file, decoded); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
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
