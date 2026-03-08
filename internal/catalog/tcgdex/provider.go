package tcgdex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

const baseURL = "https://api.tcgdex.net/v2"

var japaneseSetIDRE = regexp.MustCompile(`\d[a-z]$`)

type Provider struct {
	client *http.Client

	mu      sync.RWMutex
	setLang map[string]string
}

func New(client *http.Client) catalog.Provider {
	return &Provider{
		client:  client,
		setLang: make(map[string]string),
	}
}

func (p *Provider) Name() string {
	return "tcgdex"
}

func (p *Provider) FetchSets(ctx context.Context) ([]domain.RemoteSet, error) {
	languages := []string{"en", "ja"}
	seen := make(map[string]struct{})
	nextSetLang := make(map[string]string)
	sets := make([]domain.RemoteSet, 0, 512)

	for _, language := range languages {
		var briefs []setBriefDTO
		if err := p.getJSON(ctx, fmt.Sprintf("%s/%s/sets", baseURL, language), &briefs); err != nil {
			return nil, err
		}
		for _, brief := range briefs {
			setID := strings.TrimSpace(brief.ID)
			if setID == "" {
				continue
			}
			if language == "en" && looksJapaneseSetID(setID) {
				continue
			}
			if _, ok := seen[setID]; ok {
				continue
			}
			var detail setDetailDTO
			if err := p.getJSON(ctx, setDetailEndpoint(language, setID), &detail); err != nil {
				return nil, err
			}
			if strings.TrimSpace(detail.ID) == "" {
				detail.ID = setID
			}
			seen[setID] = struct{}{}
			nextSetLang[setID] = language
			sets = append(sets, domain.RemoteSet{
				ID:                   detail.ID,
				Language:             toLanguageLabel(language),
				Name:                 detail.Name,
				SetCode:              detail.ID,
				PriceProviderSetName: "",
				Series:               detail.Serie.Name,
				PrintedTotal:         detail.CardCount.Official,
				Total:                detail.CardCount.Total,
				ReleaseDate:          detail.ReleaseDate,
				SymbolURL:            ensurePNG(detail.Symbol),
				LogoURL:              ensurePNG(detail.Logo),
			})
		}
	}

	p.mu.Lock()
	p.setLang = nextSetLang
	p.mu.Unlock()

	return sets, nil
}

func (p *Provider) FetchCardsForSet(ctx context.Context, setID string) ([]domain.RemoteCard, error) {
	language, ok := p.lookupSetLanguage(setID)
	if !ok {
		if _, err := p.FetchSets(ctx); err != nil {
			return nil, err
		}
		language, ok = p.lookupSetLanguage(setID)
		if !ok {
			return nil, fmt.Errorf("set id %s not found in tcgdex catalog", setID)
		}
	}
	if language == "en" && looksJapaneseSetID(setID) {
		language = "ja"
	}

	var detail setDetailDTO
	if err := p.getJSON(ctx, setDetailEndpoint(language, setID), &detail); err != nil {
		return nil, err
	}

	out := make([]domain.RemoteCard, 0, len(detail.Cards))
	languageLabel := toLanguageLabel(language)
	for _, card := range detail.Cards {
		if strings.TrimSpace(card.ID) == "" {
			continue
		}
		imageBase := strings.TrimSpace(card.Image)
		out = append(out, domain.RemoteCard{
			ID:                  card.ID,
			SetID:               detail.ID,
			SetName:             detail.Name,
			SetCode:             detail.ID,
			Language:            languageLabel,
			Name:                card.Name,
			Number:              card.LocalID,
			ReleaseDate:         detail.ReleaseDate,
			PriceProviderCardID: "",
			ImageBaseURL:        imageBase,
			ImageURL:            cardImagePNG(imageBase),
		})
	}
	return out, nil
}

func (p *Provider) lookupSetLanguage(setID string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	lang, ok := p.setLang[setID]
	return lang, ok
}

func (p *Provider) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func toLanguageLabel(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "en":
		return "English"
	case "ja":
		return "Japanese"
	default:
		return strings.TrimSpace(language)
	}
}

func ensurePNG(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	base := path.Base(value)
	if strings.Contains(base, ".") {
		return value
	}
	return value + ".png"
}

func cardImagePNG(baseURL string) string {
	value := strings.TrimSpace(baseURL)
	if value == "" {
		return ""
	}
	return strings.TrimRight(value, "/") + "/high.png"
}

func setDetailEndpoint(language string, setID string) string {
	id := strings.TrimSpace(setID)
	escaped := url.PathEscape(id)
	// TCGDex expects '+' to stay literal, not decoded as a space.
	escaped = strings.ReplaceAll(escaped, "+", "%2B")
	return fmt.Sprintf("%s/%s/sets/%s", baseURL, language, escaped)
}

func looksJapaneseSetID(setID string) bool {
	id := strings.ToLower(strings.TrimSpace(setID))
	if id == "" {
		return false
	}
	return japaneseSetIDRE.MatchString(id)
}

type setBriefDTO struct {
	ID string `json:"id"`
}

type setDetailDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Logo        string `json:"logo"`
	Symbol      string `json:"symbol"`
	ReleaseDate string `json:"releaseDate"`
	Serie       struct {
		Name string `json:"name"`
	} `json:"serie"`
	CardCount struct {
		Total    int `json:"total"`
		Official int `json:"official"`
	} `json:"cardCount"`
	Cards []struct {
		ID      string `json:"id"`
		Image   string `json:"image"`
		LocalID string `json:"localId"`
		Name    string `json:"name"`
	} `json:"cards"`
}
