package pokedata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

const (
	baseURL = "https://www.pokedata.io"
)

var nextDataScriptRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type Provider struct {
	client   *http.Client
	cooldown time.Duration

	mu          sync.RWMutex
	setNameByID map[string]string
	setCodeByID map[string]string
}

func New(client *http.Client, cooldown time.Duration) catalog.Provider {
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Provider{
		client:      client,
		cooldown:    cooldown,
		setNameByID: make(map[string]string),
		setCodeByID: make(map[string]string),
	}
}

func (p *Provider) Name() string {
	return "pokedata"
}

func (p *Provider) FetchSets(ctx context.Context) ([]domain.RemoteSet, error) {
	buildID, err := p.fetchBuildID(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/_next/data/%s/sets.json", baseURL, buildID)
	var payload setsPayload
	if err := p.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}

	setInfo := payload.PageProps.SetInfoArr
	if len(setInfo) == 0 {
		setInfo = payload.Props.PageProps.SetInfoArr
	}
	sets := make([]domain.RemoteSet, 0, len(setInfo))
	nextNameMap := make(map[string]string, len(setInfo))
	nextCodeMap := make(map[string]string, len(setInfo))
	for _, dto := range setInfo {
		if !dto.Live {
			continue
		}
		id := strconv.Itoa(dto.ID)
		name := util.DecodeEscapedText(dto.Name)
		series := util.DecodeEscapedText(dto.Series)
		setCode := util.DecodeEscapedText(firstNonBlank(
			anyToString(dto.SetCode),
			anyToString(dto.SetCodeCamel),
			anyToString(dto.Code),
			dto.Abbrev,
			dto.PTCGOCode,
			dto.PTCGOCodeCamel,
		))
		nextNameMap[id] = name
		nextCodeMap[id] = setCode
		sets = append(sets, domain.RemoteSet{
			ID:           id,
			Language:     util.DecodeEscapedText(dto.Language),
			Name:         name,
			SetCode:      setCode,
			Series:       series,
			ReleaseDate:  dto.ReleaseDate,
			SymbolURL:    dto.SymbolImgURL,
			LogoURL:      dto.ImgURL,
			PrintedTotal: 0,
			Total:        0,
		})
	}

	p.mu.Lock()
	p.setNameByID = nextNameMap
	p.setCodeByID = nextCodeMap
	p.mu.Unlock()

	return sets, nil
}

func (p *Provider) FetchCardsForSet(ctx context.Context, setID string) ([]domain.RemoteCard, error) {
	setName, ok := p.getSetName(setID)
	setCode, codeOK := p.getSetCode(setID)
	if !ok {
		if _, err := p.FetchSets(ctx); err != nil {
			return nil, err
		}
		setName, ok = p.getSetName(setID)
		setCode, codeOK = p.getSetCode(setID)
		if !ok {
			return nil, fmt.Errorf("set id %s not found in provider map", setID)
		}
	}
	if !codeOK {
		setCode = ""
	}

	endpoint := fmt.Sprintf("%s/api/cards?set_name=%s", baseURL, url.QueryEscape(setName))
	var payload []cardDTO
	if err := p.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}

	cards := make([]domain.RemoteCard, 0, len(payload))
	for _, dto := range payload {
		cardID := strconv.Itoa(dto.ID)
		cardSetID := strconv.Itoa(dto.SetID)
		if cardSetID == "0" || cardSetID == "" {
			cardSetID = setID
		}
		rarity := ""
		if dto.Secret {
			rarity = "Secret"
		}
		cards = append(cards, domain.RemoteCard{
			ID:      cardID,
			SetID:   cardSetID,
			SetName: util.DecodeEscapedText(dto.SetName),
			SetCode: util.DecodeEscapedText(firstNonBlank(
				anyToString(dto.SetCode),
				anyToString(dto.SetCodeCamel),
				anyToString(dto.Code),
				setCode,
			)),
			Language:    util.DecodeEscapedText(dto.Language),
			Name:        util.DecodeEscapedText(dto.Name),
			Number:      dto.Num,
			ReleaseDate: dto.ReleaseDate,
			Secret:      dto.Secret,
			TCGPlayerID: anyToString(dto.TCGPlayerID),
			Rarity:      rarity,
			ImageURL:    dto.ImgURL,
		})
	}

	return cards, nil
}

func (p *Provider) fetchBuildID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/sets", nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	body, err := p.doRequest(ctx, req, true)
	if err != nil {
		return "", fmt.Errorf("fetch sets page: %w", err)
	}
	match := nextDataScriptRE.FindStringSubmatch(body)
	if len(match) != 2 {
		return "", fmt.Errorf("__NEXT_DATA__ script not found")
	}
	var payload struct {
		BuildID string `json:"buildId"`
	}
	if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
		return "", fmt.Errorf("decode __NEXT_DATA__: %w", err)
	}
	if payload.BuildID == "" {
		return "", fmt.Errorf("buildId missing in __NEXT_DATA__ payload")
	}
	return payload.BuildID, nil
}

func (p *Provider) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	body, err := p.doRequest(ctx, req, false)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode response %s: %w", endpoint, err)
	}
	return nil
}

func (p *Provider) doRequest(ctx context.Context, req *http.Request, noUserAgent bool) (string, error) {
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		nextReq := req.Clone(ctx)
		if !noUserAgent && nextReq.Header.Get("User-Agent") == "" {
			nextReq.Header.Set("User-Agent", "pkmn-tc-value/1.0 (+local CLI)")
		}
		resp, err := p.client.Do(nextReq)
		if err != nil {
			return "", err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt == maxAttempts {
				return "", fmt.Errorf("%s %s failed: %s", nextReq.Method, nextReq.URL.String(), http.StatusText(http.StatusTooManyRequests))
			}
			if err := p.waitCooldown(ctx); err != nil {
				return "", err
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("%s %s failed: %s", nextReq.Method, nextReq.URL.String(), resp.Status)
		}
		return string(body), nil
	}
	return "", fmt.Errorf("request retries exhausted")
}

func (p *Provider) waitCooldown(ctx context.Context) error {
	timer := time.NewTimer(p.cooldown)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p *Provider) getSetName(setID string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	name, ok := p.setNameByID[setID]
	return name, ok
}

func (p *Provider) getSetCode(setID string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	code, ok := p.setCodeByID[setID]
	return code, ok
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func anyToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

type setsPayload struct {
	PageProps struct {
		SetInfoArr []setDTO `json:"setInfoArr"`
	} `json:"pageProps"`
	Props struct {
		PageProps struct {
			SetInfoArr []setDTO `json:"setInfoArr"`
		} `json:"pageProps"`
	} `json:"props"`
}

type setDTO struct {
	ID             int    `json:"id"`
	Language       string `json:"language"`
	Live           bool   `json:"live"`
	Name           string `json:"name"`
	Code           any    `json:"code"`
	SetCode        any    `json:"set_code"`
	SetCodeCamel   any    `json:"setCode"`
	Abbrev         string `json:"abbrev"`
	PTCGOCode      string `json:"ptcgo_code"`
	PTCGOCodeCamel string `json:"ptcgoCode"`
	ReleaseDate    string `json:"release_date"`
	Series         string `json:"series"`
	SymbolImgURL   string `json:"symbol_img_url"`
	ImgURL         string `json:"img_url"`
}

type cardDTO struct {
	ID           int    `json:"id"`
	ImgURL       string `json:"img_url"`
	Language     string `json:"language"`
	Name         string `json:"name"`
	Num          string `json:"num"`
	ReleaseDate  string `json:"release_date"`
	Secret       bool   `json:"secret"`
	SetCode      any    `json:"set_code"`
	SetCodeCamel any    `json:"setCode"`
	Code         any    `json:"code"`
	SetID        int    `json:"set_id"`
	SetName      string `json:"set_name"`
	TCGPlayerID  any    `json:"tcgplayer_id"`
}
