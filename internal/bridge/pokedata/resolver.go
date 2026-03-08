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
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

const (
	baseURL = "https://www.pokedata.io"
)

var nextDataScriptRE = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type Resolver struct {
	client   *http.Client
	cooldown time.Duration

	mu    sync.RWMutex
	sets  []setDTO
	ready bool
}

type setDTO struct {
	ID       int    `json:"id"`
	Live     bool   `json:"live"`
	Language string `json:"language"`
	Name     string `json:"name"`
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

func NewResolver(client *http.Client, cooldown time.Duration) *Resolver {
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Resolver{
		client:   client,
		cooldown: cooldown,
	}
}

func (r *Resolver) MapSetCards(ctx context.Context, set domain.Set, cards []domain.RemoteCard) (string, map[string]string, error) {
	priceSetName, err := r.resolveSetName(ctx, set)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(priceSetName) == "" {
		return "", map[string]string{}, nil
	}

	priceCards, usedSetName, err := r.fetchCardsWithFallback(ctx, priceSetName, set.Name)
	if err != nil {
		return "", nil, err
	}
	matches := MatchRemoteCards(cards, priceCards)
	return usedSetName, matches, nil
}

func (r *Resolver) ResolveCardID(ctx context.Context, set domain.Set, card domain.Card) (string, string, error) {
	priceSetName, err := r.resolveSetName(ctx, set)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(priceSetName) == "" {
		return "", "", nil
	}

	priceCards, usedSetName, err := r.fetchCardsWithFallback(ctx, priceSetName, set.Name)
	if err != nil {
		return "", "", err
	}

	match := MatchLocalCard(card, priceCards)
	return match, usedSetName, nil
}

func (r *Resolver) resolveSetName(ctx context.Context, set domain.Set) (string, error) {
	if strings.TrimSpace(set.PriceProviderSetName) != "" {
		return set.PriceProviderSetName, nil
	}
	sets, err := r.fetchSets(ctx)
	if err != nil {
		return "", err
	}

	wantLang := normalizeLanguage(set.Language)
	wantName := strings.TrimSpace(set.Name)
	wantNorm := util.NormalizeName(wantName)

	type candidate struct {
		name  string
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
		if strings.EqualFold(name, wantName) {
			return name, nil
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
		candidates = append(candidates, candidate{name: name, score: score})
	}

	if len(candidates) == 0 {
		return "", nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].name, nil
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
