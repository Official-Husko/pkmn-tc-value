package pokemonpricetracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
)

const (
	v2BaseURL     = "https://www.pokemonpricetracker.com/api/v2"
	publicBaseURL = "https://www.pokemonpricetracker.com/api"
)

type Client struct {
	http   *http.Client
	keys   *KeyRing
	logger *providerslog.Logger
}

func NewClient(httpClient *http.Client, keys *KeyRing, logger *providerslog.Logger) *Client {
	return &Client{
		http:   httpClient,
		keys:   keys,
		logger: logger,
	}
}

func (c *Client) ValidateKeys(ctx context.Context, userAgent string) (ValidationSummary, error) {
	if c.keys == nil {
		return ValidationSummary{}, nil
	}
	return c.keys.Validate(ctx, c.http, userAgent)
}

func (c *Client) KeyStatuses() []KeyStatus {
	if c.keys == nil {
		return nil
	}
	return c.keys.Snapshot()
}

func (c *Client) FetchSets(ctx context.Context, language string, cfg config.Config) ([]trackerSet, error) {
	values := url.Values{}
	values.Set("language", normalizeAPILanguage(language))
	values.Set("limit", "500")
	values.Set("offset", "0")
	endpoint := v2BaseURL + "/sets?" + values.Encode()

	body, err := c.doV2(ctx, endpoint, cfg)
	if err != nil {
		return nil, err
	}
	var env setsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode sets response: %w", err)
	}
	return env.Data, nil
}

func (c *Client) FetchCardsBySetID(ctx context.Context, language string, setID string, cfg config.Config) ([]trackerCard, error) {
	return c.fetchCards(ctx, language, map[string]string{
		"setId": strings.TrimSpace(setID),
	}, cfg)
}

func (c *Client) FetchCardsBySetName(ctx context.Context, language string, setName string, cfg config.Config) ([]trackerCard, error) {
	return c.fetchCards(ctx, language, map[string]string{
		"set": strings.TrimSpace(setName),
	}, cfg)
}

func (c *Client) fetchCards(ctx context.Context, language string, filters map[string]string, cfg config.Config) ([]trackerCard, error) {
	offset := 0
	limit := 200
	all := make([]trackerCard, 0, 256)

	for {
		values := url.Values{}
		values.Set("language", normalizeAPILanguage(language))
		values.Set("limit", strconv.Itoa(limit))
		values.Set("offset", strconv.Itoa(offset))
		values.Set("includeHistory", "false")
		values.Set("includeEbay", "false")
		for key, value := range filters {
			if strings.TrimSpace(value) == "" {
				continue
			}
			values.Set(key, value)
		}

		endpoint := v2BaseURL + "/cards?" + values.Encode()
		body, err := c.doV2(ctx, endpoint, cfg)
		if err != nil {
			return nil, err
		}
		page, total, err := decodeCards(body)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < limit {
			break
		}
		offset += len(page)
		if total > 0 && offset >= total {
			break
		}
	}
	return all, nil
}

func (c *Client) FetchCardByID(ctx context.Context, language string, cardID string, includeEbay bool, cfg config.Config) (trackerCard, error) {
	values := url.Values{}
	values.Set("language", normalizeAPILanguage(language))
	values.Set("cardId", strings.TrimSpace(cardID))
	values.Set("limit", "1")
	if includeEbay {
		values.Set("includeEbay", "true")
		values.Set("days", "7")
	} else {
		values.Set("includeEbay", "false")
	}
	values.Set("includeHistory", "false")

	endpoint := v2BaseURL + "/cards?" + values.Encode()
	body, err := c.doV2(ctx, endpoint, cfg)
	if err != nil {
		return trackerCard{}, err
	}
	cards, _, err := decodeCards(body)
	if err != nil {
		return trackerCard{}, err
	}
	if len(cards) == 0 {
		return trackerCard{}, fmt.Errorf("card %s not found", cardID)
	}
	return cards[0], nil
}

func (c *Client) FetchPublicDetails(ctx context.Context, language string, cardID string, cfg config.Config) (trackerCard, error) {
	values := url.Values{}
	values.Set("days", "30")
	values.Set("language", normalizeAPILanguage(language))
	endpoint := fmt.Sprintf("%s/cards/%s/details?%s", publicBaseURL, url.PathEscape(strings.TrimSpace(cardID)), values.Encode())

	body, err := c.doPublic(ctx, endpoint, cfg)
	if err != nil {
		return trackerCard{}, err
	}
	var env detailsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return trackerCard{}, fmt.Errorf("decode card details: %w", err)
	}
	if !env.Success {
		return trackerCard{}, fmt.Errorf("card details request unsuccessful")
	}
	if strings.TrimSpace(env.Data.TCGPlayerID.String()) == "" {
		return trackerCard{}, fmt.Errorf("card details missing tcgPlayerId")
	}
	return env.Data, nil
}

func (c *Client) FetchInternalHistory(ctx context.Context, language string, cardID string, cfg config.Config) (historyEnvelope, error) {
	values := url.Values{}
	values.Set("language", normalizeAPILanguage(language))
	values.Set("cardId", strings.TrimSpace(cardID))
	values.Set("days", "7")
	endpoint := v2BaseURL + "/internal/card-history?" + values.Encode()

	body, err := c.doV2(ctx, endpoint, cfg)
	if err != nil {
		return historyEnvelope{}, err
	}
	var out historyEnvelope
	if err := json.Unmarshal(body, &out); err != nil {
		return historyEnvelope{}, fmt.Errorf("decode card history response: %w", err)
	}
	return out, nil
}

func (c *Client) doV2(ctx context.Context, endpoint string, cfg config.Config) ([]byte, error) {
	if c.keys == nil {
		return nil, ErrNoUsableAPIKey
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, _, err := c.keys.Do(ctx, c.http, req, cfg.UserAgent, 1)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if c.logger != nil {
		c.logger.LogJSON("pokemonpricetracker-v2", endpoint, body)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *Client) doPublic(ctx context.Context, endpoint string, cfg config.Config) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if strings.TrimSpace(cfg.UserAgent) != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s failed: %w", endpoint, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if c.logger != nil {
		c.logger.LogJSON("pokemonpricetracker-public", endpoint, body)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func normalizeAPILanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "japanese":
		return "japanese"
	default:
		return "english"
	}
}

func decodeCards(body []byte) ([]trackerCard, int, error) {
	var env cardsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, 0, fmt.Errorf("decode cards response: %w", err)
	}
	total := env.Metadata.Total
	if total == 0 {
		total = env.Metadata.Pagination.Total
	}
	if len(env.Data) == 0 {
		return nil, total, nil
	}

	var cards []trackerCard
	if err := json.Unmarshal(env.Data, &cards); err == nil {
		return cards, total, nil
	}
	var card trackerCard
	if err := json.Unmarshal(env.Data, &card); err == nil {
		return []trackerCard{card}, 1, nil
	}
	return nil, 0, fmt.Errorf("decode cards payload failed")
}
