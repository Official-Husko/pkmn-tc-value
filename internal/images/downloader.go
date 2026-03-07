package images

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

type Downloader struct {
	client *http.Client
	cache  *Cache
}

func NewDownloader(client *http.Client, cache *Cache) *Downloader {
	return &Downloader{client: client, cache: cache}
}

func (d *Downloader) Ensure(ctx context.Context, card domain.Card) (string, error) {
	if card.ImageURL == "" {
		return "", nil
	}
	if err := d.cache.EnsureDir(card); err != nil {
		return "", err
	}
	path := d.cache.Path(card)
	if d.cache.Exists(card) {
		return path, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, card.ImageURL, nil)
	if err != nil {
		return "", fmt.Errorf("build image request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch image failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	if err := store.WriteFileAtomically(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
