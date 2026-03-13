package images

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
)

type Downloader struct {
	client            *http.Client
	cache             *Cache
	apiKeys           []string
	userAgent         string
	backupImageSource bool
	debug             bool
	debugLogPath      string
}

var errNoImageCandidates = errors.New("no image candidates available")

func NewDownloader(client *http.Client, cache *Cache, apiKeys []string, userAgent string, backupImageSource bool, debug bool, debugLogPath string) *Downloader {
	cleanKeys := make([]string, 0, len(apiKeys))
	for _, key := range apiKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			cleanKeys = append(cleanKeys, trimmed)
		}
	}
	return &Downloader{
		client:            client,
		cache:             cache,
		apiKeys:           cleanKeys,
		userAgent:         strings.TrimSpace(userAgent),
		backupImageSource: backupImageSource,
		debug:             debug,
		debugLogPath:      debugLogPath,
	}
}

func (d *Downloader) Ensure(ctx context.Context, card domain.Card) (string, error) {
	if err := d.cache.EnsureDir(card); err != nil {
		return "", err
	}
	path := d.cache.Path(card)
	if d.cache.Exists(card) {
		return path, nil
	}
	converted, err := d.fetchCardImageAsPNG(ctx, card)
	if err != nil {
		if errors.Is(err, errNoImageCandidates) {
			return "", nil
		}
		return "", err
	}
	if err := store.WriteFileAtomically(path, converted, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (d *Downloader) FetchTempPNG(ctx context.Context, card domain.Card) (string, error) {
	converted, err := d.fetchCardImageAsPNG(ctx, card)
	if err != nil {
		if errors.Is(err, errNoImageCandidates) {
			return "", nil
		}
		return "", err
	}
	file, err := os.CreateTemp("", "pkmn-card-*.png")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.Write(converted); err != nil {
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

func (d *Downloader) fetchAsPNG(ctx context.Context, imageURL string) ([]byte, error) {
	data, err := d.fetchRawImage(ctx, imageURL)
	if err != nil {
		return nil, err
	}
	converted, err := convertToPNG(data)
	if err != nil {
		return nil, fmt.Errorf("decode and convert image: %w", err)
	}
	return converted, nil
}

func (d *Downloader) fetchRawImage(ctx context.Context, imageURL string) ([]byte, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil {
		return nil, fmt.Errorf("invalid image URL: %w", err)
	}
	if isPokewalletImageEndpoint(parsedURL) {
		return d.fetchPokewalletImage(ctx, parsedURL.String())
	}
	return d.fetchImageWithKey(ctx, parsedURL.String(), "")
}

func (d *Downloader) fetchPokewalletImage(ctx context.Context, imageURL string) ([]byte, error) {
	if len(d.apiKeys) == 0 {
		return nil, fmt.Errorf("no pokewallet api keys configured for image request")
	}
	var lastErr error
	for _, apiKey := range d.apiKeys {
		data, err := d.fetchImageWithKey(ctx, imageURL, apiKey)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("pokewallet image request failed")
}

func (d *Downloader) fetchImageWithKey(ctx context.Context, imageURL string, apiKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}
	if d.userAgent != "" {
		req.Header.Set("User-Agent", d.userAgent)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch image failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return data, nil
}

func (d *Downloader) fetchCardImageAsPNG(ctx context.Context, card domain.Card) ([]byte, error) {
	candidates := imageURLCandidates(card, d.backupImageSource)
	d.debugf(
		"[images] card=%s set=%q setCode=%q language=%q number=%q imageBase=%q backup=%t assembled_candidates=%v",
		card.ID,
		card.SetName,
		card.SetCode,
		card.Language,
		card.Number,
		card.ImageBaseURL,
		d.backupImageSource,
		candidates,
	)
	if len(candidates) == 0 {
		d.debugf("[images] card=%s no image URL candidates", card.ID)
		return nil, errNoImageCandidates
	}

	var failures []string
	for _, candidate := range candidates {
		d.debugf("[images] card=%s trying image URL: %s", card.ID, candidate)
		converted, err := d.fetchAsPNG(ctx, candidate)
		if err == nil {
			d.debugf("[images] card=%s image fetch success: %s", card.ID, candidate)
			return converted, nil
		}
		d.debugf("[images] card=%s image fetch failed: %s error=%v", card.ID, candidate, err)
		failures = append(failures, fmt.Sprintf("%s (%v)", candidate, err))
	}
	return nil, fmt.Errorf("all image sources failed: %s", strings.Join(failures, "; "))
}

func (d *Downloader) debugf(format string, args ...any) {
	if !d.debug {
		return
	}
	path := strings.TrimSpace(d.debugLogPath)
	if path == "" {
		path = "debug.log"
	}
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line)
}

func imageURLCandidates(card domain.Card, useBackup bool) []string {
	pokewalletPrimary := pokewalletImageURL(card)
	primary := tcgdexImageURL(card)

	out := make([]string, 0, 4)
	seen := make(map[string]struct{})
	appendURL := func(v string) {
		value := strings.TrimSpace(v)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	if !useBackup {
		appendURL(primary)
		// Keep image downloads free by default; authenticated Pokewallet image fetches are backup-only.
		if len(out) == 0 {
			return nil
		}
		return out
	}

	appendURL(primary)
	appendURL(pokewalletPrimary)
	appendURL(scrydexImageURL(card))
	appendURL(card.ImageURL)
	return out
}

func pokewalletImageURL(card domain.Card) string {
	id := strings.TrimSpace(card.PriceProviderCardID)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "pk_") {
		return fmt.Sprintf("https://api.pokewallet.io/images/%s?size=high", url.PathEscape(id))
	}
	// Stored provider IDs strip "pk_". Add it back for TCGPlayer-linked cards.
	if strings.TrimSpace(card.TCGPlayerID) != "" && !isDigits(id) {
		id = "pk_" + id
	}
	return fmt.Sprintf("https://api.pokewallet.io/images/%s?size=high", url.PathEscape(id))
}

func tcgdexImageURL(card domain.Card) string {
	base := strings.TrimSpace(card.ImageBaseURL)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/high.png"
}

func scrydexImageURL(card domain.Card) string {
	setName := normalizeScrydexSetCode(card.SetCode)
	number := strings.TrimSpace(card.Number)
	if setName == "" || number == "" {
		return ""
	}
	if isJapaneseLanguage(card.Language) && !strings.HasSuffix(setName, "_ja") {
		setName += "_ja"
	}
	return fmt.Sprintf("https://images.scrydex.com/pokemon/%s-%s/large", url.PathEscape(setName), url.PathEscape(number))
}

func normalizeScrydexSetCode(setCode string) string {
	code := strings.ToLower(strings.TrimSpace(setCode))
	if code == "" {
		return ""
	}
	// Scrydex set code segment is compact and lowercase.
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func isJapaneseLanguage(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	if normalized == "ja" || normalized == "jp" {
		return true
	}
	return strings.Contains(normalized, "japanese")
}

func isPokewalletImageEndpoint(parsedURL *url.URL) bool {
	if parsedURL == nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	if host != "api.pokewallet.io" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(parsedURL.Path), "/images/")
}

func isDigits(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	_, err := strconv.Atoi(trimmed)
	return err == nil
}

func convertToPNG(input []byte) ([]byte, error) {
	decoded, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, decoded); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
