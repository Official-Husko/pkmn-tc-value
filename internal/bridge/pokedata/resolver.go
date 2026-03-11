package pokedata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

const (
	baseURL = "https://www.pokedata.io"
)

var nextDataScriptRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type Resolver struct {
	client   *http.Client
	cooldown time.Duration
	logger   *providerslog.Logger

	mu    sync.RWMutex
	sets  []setDTO
	ready bool
}

type setDTO struct {
	ID       int    `json:"id"`
	Live     bool   `json:"live"`
	Language string `json:"language"`
	Name     string `json:"name"`
	Code     any    `json:"code"`
	SetCode  any    `json:"set_code"`
}

type cardDTO struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Num      string `json:"num"`
	SetName  string `json:"set_name"`
	Language string `json:"language"`
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

type PokeCard struct {
	ID       string
	Name     string
	Number   string
	SetName  string
	Language string
}

type ResolvedSet struct {
	Name string
	Code string
}

func NewResolver(client *http.Client, cooldown time.Duration, logger *providerslog.Logger) *Resolver {
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Resolver{
		client:   client,
		cooldown: cooldown,
		logger:   logger,
	}
}

func (r *Resolver) MapSetCards(ctx context.Context, set domain.Set, cards []domain.RemoteCard) (ResolvedSet, map[string]string, error) {
	resolvedSet, err := r.resolveSet(ctx, set)
	if err != nil {
		return ResolvedSet{}, nil, err
	}
	if strings.TrimSpace(resolvedSet.Name) == "" {
		return ResolvedSet{}, map[string]string{}, nil
	}

	priceCards, usedSetName, err := r.fetchCardsWithFallback(ctx, resolvedSet.Name, set.Name)
	if err != nil {
		return ResolvedSet{}, nil, err
	}
	usedCode := strings.TrimSpace(resolvedSet.Code)
	if !strings.EqualFold(strings.TrimSpace(usedSetName), strings.TrimSpace(resolvedSet.Name)) {
		usedCode = strings.TrimSpace(r.findSetCodeByName(set.Language, usedSetName))
	}
	matches := MatchRemoteCards(cards, priceCards)
	return ResolvedSet{Name: usedSetName, Code: usedCode}, matches, nil
}

func (r *Resolver) ResolveCardID(ctx context.Context, set domain.Set, card domain.Card) (string, string, string, error) {
	resolvedSet, err := r.resolveSet(ctx, set)
	if err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(resolvedSet.Name) == "" {
		return "", "", "", nil
	}

	priceCards, usedSetName, err := r.fetchCardsWithFallback(ctx, resolvedSet.Name, set.Name)
	if err != nil {
		return "", "", "", err
	}
	usedCode := strings.TrimSpace(resolvedSet.Code)
	if !strings.EqualFold(strings.TrimSpace(usedSetName), strings.TrimSpace(resolvedSet.Name)) {
		usedCode = strings.TrimSpace(r.findSetCodeByName(set.Language, usedSetName))
	}

	match := MatchLocalCard(card, priceCards)
	return match, usedSetName, usedCode, nil
}

func (r *Resolver) resolveSet(ctx context.Context, set domain.Set) (ResolvedSet, error) {
	sets, err := r.fetchSets(ctx)
	if err != nil {
		return ResolvedSet{}, err
	}

	cachedName := strings.TrimSpace(set.PriceProviderSetName)
	cachedCode := strings.TrimSpace(set.PriceProviderSetCode)
	if cachedName != "" || cachedCode != "" {
		if cachedName != "" && cachedCode != "" {
			return ResolvedSet{Name: cachedName, Code: cachedCode}, nil
		}
		codeTarget := normalizeCode(cachedCode)
		for _, item := range sets {
			if !item.Live {
				continue
			}
			name := util.DecodeEscapedText(item.Name)
			code := setCodeOf(item)
			if cachedName != "" && strings.EqualFold(name, cachedName) {
				if strings.TrimSpace(cachedCode) == "" {
					return ResolvedSet{Name: name, Code: code}, nil
				}
				return ResolvedSet{Name: name, Code: cachedCode}, nil
			}
			if codeTarget != "" && normalizeCode(code) == codeTarget {
				if strings.TrimSpace(cachedName) == "" {
					return ResolvedSet{Name: name, Code: code}, nil
				}
				return ResolvedSet{Name: cachedName, Code: code}, nil
			}
		}
		return ResolvedSet{Name: cachedName, Code: cachedCode}, nil
	}

	codeCandidates := []string{
		normalizeCode(set.SetCode),
		normalizeCode(set.ID),
	}
	wantLang := normalizeLanguage(set.Language)
	for _, target := range codeCandidates {
		if target == "" {
			continue
		}
		// Prefer language match first.
		for _, item := range sets {
			if !item.Live {
				continue
			}
			if wantLang != "" && normalizeLanguage(item.Language) != wantLang {
				continue
			}
			code := setCodeOf(item)
			if normalizeCode(code) == target {
				return ResolvedSet{Name: util.DecodeEscapedText(item.Name), Code: code}, nil
			}
		}
		// Then allow cross-language if no same-language hit.
		for _, item := range sets {
			if !item.Live {
				continue
			}
			code := setCodeOf(item)
			if normalizeCode(code) == target {
				return ResolvedSet{Name: util.DecodeEscapedText(item.Name), Code: code}, nil
			}
		}
	}

	wantName := strings.TrimSpace(set.Name)
	wantNorm := util.NormalizeName(wantName)

	type candidate struct {
		name  string
		code  string
		score int
	}
	candidates := make([]candidate, 0, 8)
	for _, item := range sets {
		if !item.Live {
			continue
		}
		if wantLang != "" && normalizeLanguage(item.Language) != wantLang {
			continue
		}

		name := util.DecodeEscapedText(item.Name)
		code := setCodeOf(item)
		if strings.EqualFold(name, wantName) {
			return ResolvedSet{Name: name, Code: code}, nil
		}

		norm := util.NormalizeName(name)
		score := 0
		switch {
		case norm == wantNorm:
			score = 100
		case strings.Contains(norm, wantNorm) || strings.Contains(wantNorm, norm):
			score = 50
		default:
			continue
		}
		candidates = append(candidates, candidate{name: name, code: code, score: score})
	}

	if len(candidates) == 0 {
		return ResolvedSet{}, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].score > candidates[j].score
	})
	return ResolvedSet{Name: candidates[0].name, Code: candidates[0].code}, nil
}

func (r *Resolver) fetchCardsWithFallback(ctx context.Context, preferredName string, fallbackName string) ([]PokeCard, string, error) {
	primaryCards, primaryErr := r.fetchCardsBySetName(ctx, preferredName)
	if primaryErr == nil {
		return primaryCards, preferredName, nil
	}
	if strings.EqualFold(strings.TrimSpace(preferredName), strings.TrimSpace(fallbackName)) || strings.TrimSpace(fallbackName) == "" {
		return nil, "", primaryErr
	}
	fallbackCards, fallbackErr := r.fetchCardsBySetName(ctx, fallbackName)
	if fallbackErr != nil {
		return nil, "", primaryErr
	}
	return fallbackCards, fallbackName, nil
}

func (r *Resolver) fetchSets(ctx context.Context) ([]setDTO, error) {
	r.mu.RLock()
	if r.ready {
		out := make([]setDTO, len(r.sets))
		copy(out, r.sets)
		r.mu.RUnlock()
		return out, nil
	}
	r.mu.RUnlock()

	buildID, err := r.fetchBuildID(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/_next/data/%s/sets.json", baseURL, buildID)
	var payload setsPayload
	if err := r.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	sets := payload.PageProps.SetInfoArr
	if len(sets) == 0 {
		sets = payload.Props.PageProps.SetInfoArr
	}

	r.mu.Lock()
	r.sets = sets
	r.ready = true
	r.mu.Unlock()

	out := make([]setDTO, len(sets))
	copy(out, sets)
	return out, nil
}

func (r *Resolver) findSetCodeByName(language string, setName string) string {
	targetName := strings.TrimSpace(setName)
	if targetName == "" {
		return ""
	}
	targetLang := normalizeLanguage(language)

	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.sets {
		if !item.Live {
			continue
		}
		if targetLang != "" && normalizeLanguage(item.Language) != targetLang {
			continue
		}
		name := util.DecodeEscapedText(item.Name)
		if strings.EqualFold(strings.TrimSpace(name), targetName) {
			return setCodeOf(item)
		}
	}
	return ""
}

func (r *Resolver) fetchCardsBySetName(ctx context.Context, setName string) ([]PokeCard, error) {
	endpoint := fmt.Sprintf("%s/api/cards?set_name=%s", baseURL, url.QueryEscape(setName))
	var payload []cardDTO
	if err := r.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	out := make([]PokeCard, 0, len(payload))
	for _, dto := range payload {
		out = append(out, PokeCard{
			ID:       strconv.Itoa(dto.ID),
			Name:     util.DecodeEscapedText(dto.Name),
			Number:   strings.TrimSpace(dto.Num),
			SetName:  util.DecodeEscapedText(dto.SetName),
			Language: util.DecodeEscapedText(dto.Language),
		})
	}
	return out, nil
}

func (r *Resolver) fetchBuildID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/sets", nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	body, err := r.doRequest(ctx, req, true)
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

func (r *Resolver) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	body, err := r.doRequest(ctx, req, false)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	if r.logger != nil {
		r.logger.LogJSON("pokedata-bridge", endpoint, []byte(body))
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode response %s: %w", endpoint, err)
	}
	return nil
}

func (r *Resolver) doRequest(ctx context.Context, req *http.Request, noUserAgent bool) (string, error) {
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		nextReq := req.Clone(ctx)
		if !noUserAgent && nextReq.Header.Get("User-Agent") == "" {
			nextReq.Header.Set("User-Agent", "pkmn-tc-value/1.0 (+local CLI)")
		}
		resp, err := r.client.Do(nextReq)
		if err != nil {
			return "", err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt == maxAttempts {
				return "", fmt.Errorf("%s %s failed: %s", nextReq.Method, nextReq.URL.String(), http.StatusText(http.StatusTooManyRequests))
			}
			if err := r.waitCooldown(ctx); err != nil {
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

func (r *Resolver) waitCooldown(ctx context.Context) error {
	timer := time.NewTimer(r.cooldown)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeLanguage(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "en", "english":
		return "en"
	case "ja", "jp", "japanese":
		return "ja"
	default:
		return normalized
	}
}

func setCodeOf(set setDTO) string {
	return util.DecodeEscapedText(firstNonBlank(
		anyToString(set.SetCode),
		anyToString(set.Code),
	))
}

func normalizeCode(value string) string {
	code := strings.ToLower(strings.TrimSpace(value))
	code = strings.ReplaceAll(code, " ", "")
	return code
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
