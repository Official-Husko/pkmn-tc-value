package pokemonpricetracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
)

const (
	pokewalletBaseURL        = "https://api.pokewallet.io"
	pokepricePublicBaseURL   = "https://www.pokemonpricetracker.com/api"
	pokepricePublicV2BaseURL = "https://www.pokemonpricetracker.com/api/v2"
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
	endpoint := pokewalletBaseURL + "/sets"
	body, err := c.doAuthed(ctx, endpoint, cfg)
	if err != nil {
		return nil, err
	}
	var env setsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode sets response: %w", err)
	}

	wantLang := normalizeWalletLanguage(language)
	sets := make([]trackerSet, 0, len(env.Data))
	for _, set := range env.Data {
		lang := normalizeWalletLanguageCode(set.Language)
		if wantLang != "" && lang != "" && wantLang != lang {
			continue
		}
		setID := strings.TrimSpace(set.SetID.String())
		sets = append(sets, trackerSet{
			ID:          stringValue(setID),
			TCGPlayerID: stringValue(setID),
			Name:        strings.TrimSpace(set.Name),
			Series:      "",
			ReleaseDate: strings.TrimSpace(set.ReleaseDate),
			CardCount:   set.CardCount,
			Language:    normalizeAPILanguageFromWallet(lang),
			SetCode:     strings.TrimSpace(set.SetCode),
		})
	}
	sort.SliceStable(sets, func(i, j int) bool {
		if sets[i].ReleaseDate == sets[j].ReleaseDate {
			return sets[i].Name < sets[j].Name
		}
		return sets[i].ReleaseDate > sets[j].ReleaseDate
	})
	return sets, nil
}

func (c *Client) FetchCardsBySetID(ctx context.Context, language string, setID string, cfg config.Config) ([]trackerCard, error) {
	return c.fetchSetCardsByIdentifier(ctx, language, strings.TrimSpace(setID), cfg)
}

func (c *Client) FetchCardsBySetName(ctx context.Context, language string, setName string, cfg config.Config) ([]trackerCard, error) {
	targetName := strings.TrimSpace(setName)
	if targetName == "" {
		return nil, nil
	}
	sets, err := c.FetchSets(ctx, language, cfg)
	if err != nil {
		return nil, err
	}
	normalizedTarget := strings.ToLower(targetName)
	var match trackerSet
	found := false
	for _, set := range sets {
		name := strings.TrimSpace(set.Name)
		if strings.EqualFold(name, targetName) {
			match = set
			found = true
			break
		}
	}
	if !found {
		for _, set := range sets {
			name := strings.ToLower(strings.TrimSpace(set.Name))
			if strings.Contains(name, normalizedTarget) || strings.Contains(normalizedTarget, name) {
				match = set
				found = true
				break
			}
		}
	}
	if !found {
		return nil, nil
	}
	return c.FetchCardsBySetID(ctx, language, match.ID.String(), cfg)
}

func (c *Client) fetchSetCardsByIdentifier(ctx context.Context, language string, identifier string, cfg config.Config) ([]trackerCard, error) {
	if strings.TrimSpace(identifier) == "" {
		return nil, nil
	}

	limit := 200
	page := 1
	lang := normalizeWalletLanguage(language)
	all := make([]trackerCard, 0, limit)

	for {
		values := url.Values{}
		values.Set("page", strconv.Itoa(page))
		values.Set("limit", strconv.Itoa(limit))
		if lang != "" {
			values.Set("language", lang)
		}

		endpoint := fmt.Sprintf("%s/sets/%s?%s", pokewalletBaseURL, url.PathEscape(identifier), values.Encode())
		body, err := c.doAuthed(ctx, endpoint, cfg)
		if err != nil {
			return nil, err
		}

		var env cardsEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode set cards response: %w", err)
		}

		if env.Disambiguation && len(env.Matches) > 0 {
			picked := pickDisambiguatedSetID(env.Matches, lang)
			if picked == "" {
				return nil, fmt.Errorf("set %s is ambiguous", identifier)
			}
			if picked == identifier {
				return nil, fmt.Errorf("set %s is ambiguous", identifier)
			}
			return c.fetchSetCardsByIdentifier(ctx, language, picked, cfg)
		}

		for _, card := range env.Cards {
			all = append(all, walletToTrackerCard(card, env.Set))
		}

		if len(env.Cards) < limit {
			break
		}
		page++
		if page > 25 {
			break
		}
	}
	return all, nil
}

func (c *Client) FetchCardByID(ctx context.Context, language string, cardID string, includeEbay bool, cfg config.Config) (trackerCard, error) {
	values := url.Values{}
	walletLang := normalizeWalletLanguage(language)
	if walletLang != "" {
		values.Set("language", walletLang)
	}
	endpoint := fmt.Sprintf("%s/cards/%s", pokewalletBaseURL, url.PathEscape(strings.TrimSpace(cardID)))
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	body, err := c.doAuthed(ctx, endpoint, cfg)
	if err != nil {
		return trackerCard{}, err
	}

	var direct walletCard
	if err := json.Unmarshal(body, &direct); err == nil && strings.TrimSpace(direct.ID) != "" {
		return walletToTrackerCard(direct, struct {
			Name       string      `json:"name"`
			SetCode    string      `json:"set_code"`
			SetID      stringValue `json:"set_id"`
			TotalCards int         `json:"total_cards"`
			Language   string      `json:"language"`
		}{}), nil
	}

	var wrapped struct {
		Success bool       `json:"success"`
		Data    walletCard `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return trackerCard{}, fmt.Errorf("decode card response: %w", err)
	}
	if strings.TrimSpace(wrapped.Data.ID) == "" {
		return trackerCard{}, fmt.Errorf("card %s not found", cardID)
	}
	return walletToTrackerCard(wrapped.Data, struct {
		Name       string      `json:"name"`
		SetCode    string      `json:"set_code"`
		SetID      stringValue `json:"set_id"`
		TotalCards int         `json:"total_cards"`
		Language   string      `json:"language"`
	}{}), nil
}

func (c *Client) FetchPublicDetails(ctx context.Context, language string, cardID string, cfg config.Config) (trackerCard, error) {
	values := url.Values{}
	values.Set("days", "30")
	values.Set("language", normalizeAPILanguage(language))
	endpoint := fmt.Sprintf("%s/cards/%s/details?%s", pokepricePublicBaseURL, url.PathEscape(strings.TrimSpace(cardID)), values.Encode())

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
	endpoint := pokepricePublicV2BaseURL + "/internal/card-history?" + values.Encode()

	body, err := c.doPublic(ctx, endpoint, cfg)
	if err != nil {
		return historyEnvelope{}, err
	}
	var out historyEnvelope
	if err := json.Unmarshal(body, &out); err != nil {
		return historyEnvelope{}, fmt.Errorf("decode card history response: %w", err)
	}
	return out, nil
}

func (c *Client) doAuthed(ctx context.Context, endpoint string, cfg config.Config) ([]byte, error) {
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
		c.logger.LogHTTP("pokewallet", endpoint, resp.StatusCode, resp.Status, body)
		c.logger.LogJSON("pokewallet", endpoint, body)
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
		c.logger.LogHTTP("pokemonpricetracker-public", endpoint, resp.StatusCode, resp.Status, body)
		c.logger.LogJSON("pokemonpricetracker-public", endpoint, body)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func normalizeAPILanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "japanese", "jap":
		return "japanese"
	default:
		return "english"
	}
}

func normalizeWalletLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "japanese", "jap":
		return "jap"
	case "english", "en", "eng":
		return "eng"
	default:
		return ""
	}
}

func normalizeWalletLanguageCode(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "ja", "japanese", "jap":
		return "jap"
	case "en", "english", "eng":
		return "eng"
	default:
		return ""
	}
}

func normalizeAPILanguageFromWallet(code string) string {
	if normalizeWalletLanguageCode(code) == "jap" {
		return "japanese"
	}
	return "english"
}

func pickDisambiguatedSetID(matches []walletSet, walletLanguage string) string {
	if len(matches) == 0 {
		return ""
	}
	for _, match := range matches {
		if walletLanguage != "" && normalizeWalletLanguageCode(match.Language) == walletLanguage {
			return strings.TrimSpace(match.SetID.String())
		}
	}
	return strings.TrimSpace(matches[0].SetID.String())
}

func walletToTrackerCard(card walletCard, setMeta struct {
	Name       string      `json:"name"`
	SetCode    string      `json:"set_code"`
	SetID      stringValue `json:"set_id"`
	TotalCards int         `json:"total_cards"`
	Language   string      `json:"language"`
}) trackerCard {
	number := strings.TrimSpace(card.CardInfo.CardNumber)
	total := strings.TrimSpace(card.CardInfo.TotalSet)
	if total == "" {
		if slash := strings.Index(number, "/"); slash > 0 && slash+1 < len(number) {
			total = strings.TrimSpace(number[slash+1:])
		}
	}

	variants := make(map[string]variantPrice)
	priceData := trackerPriceData{}
	tcgURL := ""

	if card.TCGPlayer != nil {
		tcgURL = strings.TrimSpace(card.TCGPlayer.URL)
		for _, item := range card.TCGPlayer.Prices {
			name := strings.TrimSpace(item.SubTypeName)
			if name == "" {
				name = "Normal"
			}
			variants[name] = variantPrice{
				Printing:    name,
				MarketPrice: firstNumber(item.MarketPrice, item.MidPrice, item.LowPrice),
				LowPrice:    item.LowPrice,
			}
			if strings.EqualFold(name, "normal") {
				priceData.Market = firstNumber(item.MarketPrice, item.MidPrice, item.LowPrice)
				priceData.LowPrice = item.LowPrice
				priceData.LastUpdated = strings.TrimSpace(item.UpdatedAt)
			}
			if priceData.Market == nil {
				priceData.Market = firstNumber(item.MarketPrice, item.MidPrice, item.LowPrice)
			}
			if priceData.LowPrice == nil {
				priceData.LowPrice = item.LowPrice
			}
			if priceData.LastUpdated == "" {
				priceData.LastUpdated = strings.TrimSpace(item.UpdatedAt)
			}
		}
	}

	if (priceData.Market == nil || priceData.LowPrice == nil) && card.CardMarket != nil {
		for _, item := range card.CardMarket.Prices {
			if priceData.Market == nil {
				priceData.Market = firstNumber(item.Avg, item.Trend, item.Low)
			}
			if priceData.LowPrice == nil {
				priceData.LowPrice = item.Low
			}
			if priceData.LastUpdated == "" {
				priceData.LastUpdated = strings.TrimSpace(item.UpdatedAt)
			}
		}
	}

	setID := firstNonEmpty(card.CardInfo.SetID.String(), setMeta.SetID.String())
	setName := firstNonEmpty(card.CardInfo.SetName, setMeta.Name)
	language := normalizeAPILanguageFromWallet(setMeta.Language)

	return trackerCard{
		ID:             stringValue(strings.TrimSpace(card.ID)),
		TCGPlayerID:    stringValue(extractTCGPlayerID(tcgURL)),
		SetID:          stringValue(setID),
		SetName:        setName,
		Name:           firstNonEmpty(card.CardInfo.CleanName, card.CardInfo.Name),
		CardNumber:     number,
		TotalSetNumber: total,
		Rarity:         strings.TrimSpace(card.CardInfo.Rarity),
		CardType:       strings.TrimSpace(card.CardInfo.CardType),
		Artist:         "",
		Language:       language,
		Prices:         priceData,
		Variants:       variants,
	}
}

func extractTCGPlayerID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := range parts {
		if parts[i] != "product" {
			continue
		}
		if i+1 < len(parts) {
			id := strings.TrimSpace(parts[i+1])
			if _, err := strconv.Atoi(id); err == nil {
				return id
			}
		}
	}
	return ""
}
