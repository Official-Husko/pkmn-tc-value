package pokedata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
)

const statsEndpoint = "https://www.pokedata.io/api/cards/stats"

type Provider struct {
	client   *http.Client
	resolver cardIDResolver
	endpoint string
	logger   *providerslog.Logger

	mu          sync.Mutex
	lastRequest time.Time
}

type cardIDResolver interface {
	ResolveCardID(ctx context.Context, set domain.Set, card domain.Card) (string, string, string, error)
}

func New(client *http.Client, resolver cardIDResolver, logger *providerslog.Logger) pricing.Provider {
	return &Provider{client: client, resolver: resolver, endpoint: statsEndpoint, logger: logger}
}

func (p *Provider) Name() string {
	return "pokedata"
}

func (p *Provider) RefreshCard(ctx context.Context, card domain.Card, set domain.Set, cfg config.Config) (domain.PriceSnapshot, error) {
	priceProviderCardID := strings.TrimSpace(card.PriceProviderCardID)
	resolvedSetName := ""
	resolvedSetCode := ""
	if priceProviderCardID == "" && p.resolver != nil {
		resolvedID, resolvedSetNameValue, resolvedSetCodeValue, resolveErr := p.resolver.ResolveCardID(ctx, set, card)
		if resolveErr != nil {
			return domain.PriceSnapshot{}, resolveErr
		}
		priceProviderCardID = strings.TrimSpace(resolvedID)
		resolvedSetName = strings.TrimSpace(resolvedSetNameValue)
		resolvedSetCode = strings.TrimSpace(resolvedSetCodeValue)
	}
	if priceProviderCardID == "" {
		return domain.PriceSnapshot{}, fmt.Errorf(
			"missing price provider card id for card=%s number=%s set=%s setCode=%s priceSet=%s priceSetCode=%s",
			card.ID,
			card.Number,
			set.Name,
			set.SetCode,
			set.PriceProviderSetName,
			set.PriceProviderSetCode,
		)
	}

	cardID, err := strconv.Atoi(priceProviderCardID)
	if err != nil {
		return domain.PriceSnapshot{}, fmt.Errorf("parse provider card id %q: %w", priceProviderCardID, err)
	}

	if err := p.wait(ctx, cfg.RequestDelayMs); err != nil {
		return domain.PriceSnapshot{}, err
	}

	values := url.Values{}
	values.Set("id", strconv.Itoa(cardID))

	endpoint := p.endpoint + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.PriceSnapshot{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	var entries []statsEntry
	if err := p.fetchStatsWithRetry(ctx, req, cfg.RateLimitCooldownSeconds, &entries); err != nil {
		return domain.PriceSnapshot{}, err
	}

	checkedAt := time.Now().UTC()
	if latest := latestUpdatedAt(entries); !latest.IsZero() {
		checkedAt = latest
	}

	ungraded := pickSource(entries, 11.0)
	if ungraded == nil {
		ungraded = pickSource(entries, 12.0)
	}
	psa10 := pickSource(entries, 10.0)

	return domain.PriceSnapshot{
		Ungraded:             ungraded,
		PSA10:                psa10,
		SourceURL:            endpoint,
		CheckedAt:            checkedAt,
		PriceProviderCardID:  priceProviderCardID,
		PriceProviderSetName: resolvedSetName,
		PriceProviderSetCode: resolvedSetCode,
	}, nil
}

func (p *Provider) fetchStatsWithRetry(ctx context.Context, req *http.Request, cooldownSeconds int, target *[]statsEntry) error {
	const maxAttempts = 3
	cooldown := time.Duration(cooldownSeconds) * time.Second
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := p.client.Do(req.Clone(ctx))
		if err != nil {
			return fmt.Errorf("request stats: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt == maxAttempts {
				return fmt.Errorf("request %s failed: %s", req.URL.String(), resp.Status)
			}
			timer := time.NewTimer(cooldown)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if p.logger != nil {
				p.logger.LogJSON("pokedata-pricing", req.URL.String(), body)
			}
			return fmt.Errorf("request %s failed: %s (%s)", req.URL.String(), resp.Status, strings.TrimSpace(string(body)))
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read stats response: %w", readErr)
		}
		if p.logger != nil {
			p.logger.LogJSON("pokedata-pricing", req.URL.String(), body)
		}
		decErr := json.Unmarshal(body, target)
		if decErr != nil {
			return fmt.Errorf("decode stats response: %w", decErr)
		}
		return nil
	}

	return fmt.Errorf("request retries exhausted")
}

func (p *Provider) wait(ctx context.Context, delayMs int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delay := time.Duration(delayMs) * time.Millisecond
	waitFor := delay - time.Since(p.lastRequest)
	if waitFor > 0 {
		timer := time.NewTimer(waitFor)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	p.lastRequest = time.Now()
	return nil
}

func pickSource(entries []statsEntry, source float64) *domain.Money {
	for _, entry := range entries {
		if entry.Source == source && entry.Avg != nil {
			return &domain.Money{
				Amount:   *entry.Avg,
				Currency: "USD",
			}
		}
	}
	return nil
}

func latestUpdatedAt(entries []statsEntry) time.Time {
	var latest time.Time
	for _, entry := range entries {
		if entry.UpdatedAt == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC1123, entry.UpdatedAt)
		if err != nil {
			continue
		}
		if parsed.After(latest) {
			latest = parsed.UTC()
		}
	}
	return latest
}

type statsEntry struct {
	Avg       *float64 `json:"avg"`
	Source    float64  `json:"source"`
	UpdatedAt string   `json:"updated_at"`
}
