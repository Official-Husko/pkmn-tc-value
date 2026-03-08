package screens

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	_ "golang.org/x/image/webp"
)

const imageCompatTestURL = "https://pokemoncardimages.pokedata.io/images/Shiny+Treasure+ex/349.webp"

func EditSettings(cfg config.Config, renderer images.Renderer, colors bool) (config.Config, error) {
	next := cfg
	for {
		choice, canceled, err := runSelect(
			"Settings",
			"Select one option to edit.",
			settingsMenuOptions(next),
			colors,
			false,
			16,
		)
		if err != nil {
			return cfg, err
		}
		if canceled {
			return cfg, nil
		}

		switch choice {
		case "startup_sync":
			value, canceled, err := editBoolSetting(colors, "Startup Sync", "Fetch the latest set catalog when the app starts.", next.StartupSyncEnabled)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.StartupSyncEnabled = value
		case "card_refresh_ttl":
			value, canceled, err := editIntSetting(colors, "Card Refresh TTL (hours)", "Allowed range: 1 to 168.", next.CardRefreshTTLHours, 1, 168)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.CardRefreshTTLHours = value
		case "image_previews":
			value, canceled, err := editBoolSetting(colors, "Image Previews", "Render card images in the detail view when terminal support is available.", next.ImagePreviewsEnabled)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.ImagePreviewsEnabled = value
		case "image_compat":
			visible, canceled, err := runImageCompatibilityTest(renderer, colors)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.ImagePreviewsEnabled = visible
			next.ImageCaching = visible
		case "image_caching":
			value, canceled, err := editBoolSetting(colors, "Image Caching", "Store converted PNG card images in the local cache for faster future views.", next.ImageCaching)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.ImageCaching = value
		case "sync_card_details":
			value, canceled, err := editBoolSetting(colors, "Sync Card Details (prices)", "When syncing a set database, also fetch per-card detail stats and prices.", next.SyncCardDetails)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.SyncCardDetails = value
		case "colors":
			value, canceled, err := editBoolSetting(colors, "Colors", "Enable themed colors in menus, labels, and status messages.", next.ColorsEnabled)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.ColorsEnabled = value
		case "request_delay":
			value, canceled, err := editIntSetting(colors, "Request Delay (ms)", "Allowed range: 250 to 10000.", next.RequestDelayMs, 250, 10000)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.RequestDelayMs = value
		case "rate_limit_cooldown":
			value, canceled, err := editIntSetting(colors, "Rate-limit Cooldown (seconds)", "Allowed range: 1 to 300.", next.RateLimitCooldownSeconds, 1, 300)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.RateLimitCooldownSeconds = value
		case "save_searched":
			value, canceled, err := editBoolSetting(colors, "Save Searched Cards by Default", "When true, Enter in card detail defaults to Add to collection.", next.SaveSearchedCardsDefault)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.SaveSearchedCardsDefault = value
		case "user_agent":
			value, canceled, err := editTextSetting(colors, "HTTP User Agent", "User-Agent header sent to remote APIs and scraper requests.", next.UserAgent, false)
			if err != nil {
				return cfg, err
			}
			if canceled {
				continue
			}
			next.UserAgent = value
		case "save_back":
			if err := next.Validate(); err != nil {
				if showErr := ShowMessage("Invalid Settings", err.Error(), colors); showErr != nil {
					return cfg, showErr
				}
				continue
			}
			return next, nil
		case "back_no_save":
			return cfg, nil
		}
	}
}

func settingsMenuOptions(cfg config.Config) []SelectOption {
	return []SelectOption{
		{Label: "Startup sync: " + onOff(cfg.StartupSyncEnabled), Value: "startup_sync"},
		{Label: fmt.Sprintf("Card refresh TTL: %d hours", cfg.CardRefreshTTLHours), Value: "card_refresh_ttl"},
		{Label: "Image previews: " + onOff(cfg.ImagePreviewsEnabled), Value: "image_previews"},
		{Label: "Test image compatibility", Value: "image_compat"},
		{Label: "Image caching: " + onOff(cfg.ImageCaching), Value: "image_caching"},
		{Label: "Sync card details: " + onOff(cfg.SyncCardDetails), Value: "sync_card_details"},
		{Label: "Colors: " + onOff(cfg.ColorsEnabled), Value: "colors"},
		{Label: fmt.Sprintf("Request delay: %d ms", cfg.RequestDelayMs), Value: "request_delay"},
		{Label: fmt.Sprintf("Rate-limit cooldown: %d sec", cfg.RateLimitCooldownSeconds), Value: "rate_limit_cooldown"},
		{Label: "Save searched cards by default: " + onOff(cfg.SaveSearchedCardsDefault), Value: "save_searched"},
		{Label: "HTTP user agent: " + cfg.UserAgent, Value: "user_agent"},
		{Label: "Save and Back", Value: "save_back"},
		{Label: "Back without saving", Value: "back_no_save"},
	}
}

func editBoolSetting(colors bool, title string, description string, current bool) (bool, bool, error) {
	out, canceled, err := runSelect(
		title,
		description,
		[]SelectOption{
			{Label: "Enabled", Value: "true"},
			{Label: "Disabled", Value: "false"},
		},
		colors,
		false,
		8,
	)
	if err != nil {
		return current, false, err
	}
	if canceled {
		return current, true, nil
	}
	return out == "true", false, nil
}

func editIntSetting(colors bool, title string, description string, current int, min int, max int) (int, bool, error) {
	value := strconv.Itoa(current)
	for {
		out, canceled, err := runTextInput(title, description, value, colors)
		if err != nil {
			return current, false, err
		}
		if canceled {
			return current, true, nil
		}

		parsed, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			if showErr := ShowMessage("Invalid Number", "Enter digits only.", colors); showErr != nil {
				return current, false, showErr
			}
			value = out
			continue
		}
		if parsed < min || parsed > max {
			msg := fmt.Sprintf("Value must be between %d and %d.", min, max)
			if showErr := ShowMessage("Out of Range", msg, colors); showErr != nil {
				return current, false, showErr
			}
			value = out
			continue
		}
		return parsed, false, nil
	}
}

func editTextSetting(colors bool, title string, description string, current string, allowBlank bool) (string, bool, error) {
	value := current
	for {
		out, canceled, err := runTextInput(title, description, value, colors)
		if err != nil {
			return current, false, err
		}
		if canceled {
			return current, true, nil
		}
		trimmed := strings.TrimSpace(out)
		if trimmed == "" && !allowBlank {
			if showErr := ShowMessage("Invalid Value", "This value cannot be blank.", colors); showErr != nil {
				return current, false, showErr
			}
			value = out
			continue
		}
		return trimmed, false, nil
	}
}

func onOff(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

func runImageCompatibilityTest(renderer images.Renderer, colors bool) (bool, bool, error) {
	if renderer == nil {
		return false, false, ShowMessage("Image Compatibility", "Renderer is unavailable.", colors)
	}

	path, err := downloadCompatibilityImage(imageCompatTestURL)
	if err != nil {
		return false, false, err
	}
	defer os.Remove(path)

	rendered, renderErr := renderer.Render(path, 32, 12)
	if rendered == "" {
		rendered = "[image unavailable]"
	}

	description := "If you can see the sample image below, select Visible.\n\n" + rendered
	if renderErr != nil {
		description += "\n\nRenderer error: " + renderErr.Error()
	}
	if err := ShowMessage("Image Compatibility Test", description, colors); err != nil {
		return false, false, err
	}

	out, canceled, err := runSelect(
		"Was the image visible?",
		"This updates image previews and image caching automatically.",
		[]SelectOption{
			{Label: "Visible", Value: "visible"},
			{Label: "Not visible", Value: "not_visible"},
		},
		colors,
		false,
		8,
	)
	if err != nil {
		return false, false, err
	}
	if canceled {
		return false, true, nil
	}
	return out == "visible", false, nil
}

func downloadCompatibilityImage(url string) (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download test image returned %s", resp.Status)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp("", "pkmn-termimg-test-*.png")
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", err
	}
	return file.Name(), nil
}
