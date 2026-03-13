package pokemonpricetracker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

const (
	defaultDailyLimit = 1000
	defaultProbeURL   = "https://api.pokewallet.io/sets"
)

var ErrNoUsableAPIKey = errors.New("no usable API keys available")

type keyUsageRepo interface {
	UsageForDay(fingerprint string, day string) (domain.APIKeyUsage, bool, error)
	IncrementUsage(fingerprint string, day string, delta int) (domain.APIKeyUsage, error)
}

type KeyStatus struct {
	Fingerprint string
	Masked      string
	Used        int
	DailyLimit  int
	Usable      bool
	Reason      string
}

type ValidationSummary struct {
	Total     int
	Usable    int
	Invalid   int
	Exhausted int
	Blocked   int
	Statuses  []KeyStatus
}

type keyState struct {
	raw         string
	fingerprint string
	masked      string
	used        int
	usable      bool
	reason      string
	cooldownEnd time.Time
}

type KeyRing struct {
	mu         sync.Mutex
	keys       []keyState
	next       int
	dailyLimit int
	repo       keyUsageRepo
}

func NewKeyRing(keys []string, dailyLimit int, repo keyUsageRepo) *KeyRing {
	if dailyLimit < 1 {
		dailyLimit = defaultDailyLimit
	}
	states := make([]keyState, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		states = append(states, keyState{
			raw:         trimmed,
			fingerprint: fingerprintKey(trimmed),
			masked:      maskKey(trimmed),
			usable:      true,
			reason:      "not validated",
		})
	}
	return &KeyRing{
		keys:       states,
		dailyLimit: dailyLimit,
		repo:       repo,
	}
}

func (k *KeyRing) Validate(ctx context.Context, client *http.Client, userAgent string) (ValidationSummary, error) {
	k.mu.Lock()
	for i := range k.keys {
		k.keys[i].usable = false
		k.keys[i].reason = "not validated"
		k.keys[i].cooldownEnd = time.Time{}
	}
	k.next = 0
	k.mu.Unlock()

	day := currentDayUTC()
	for idx := range k.keys {
		state := k.getKey(idx)
		used, err := k.loadUsedForDay(state.fingerprint, day)
		if err != nil {
			return ValidationSummary{}, err
		}
		k.setUsed(idx, used)
		if used >= k.dailyLimit {
			k.setState(idx, false, "daily limit reached")
			continue
		}

		probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultProbeURL, nil)
		if err != nil {
			return ValidationSummary{}, fmt.Errorf("build probe request: %w", err)
		}
		if strings.TrimSpace(userAgent) != "" {
			probeReq.Header.Set("User-Agent", userAgent)
		}
		probeReq.Header.Set("X-API-Key", state.raw)
		probeReq.Header.Set("Authorization", "Bearer "+state.raw)

		resp, err := client.Do(probeReq)
		if err != nil {
			k.setState(idx, false, "probe request failed")
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			k.setState(idx, false, "invalid key")
			continue
		}
		if isQuotaLikeResponse(resp.StatusCode, body) {
			k.setState(idx, false, "quota exceeded")
			continue
		}
		if resp.StatusCode >= 300 {
			k.setState(idx, false, "probe failed")
			continue
		}
		if _, err := k.incrementUsage(state.fingerprint, day, 1); err != nil {
			return ValidationSummary{}, err
		}
		k.setUsed(idx, used+1)
		k.setState(idx, true, "ok")
	}

	return k.Summary(), nil
}

func (k *KeyRing) Summary() ValidationSummary {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := ValidationSummary{
		Total:    len(k.keys),
		Statuses: make([]KeyStatus, 0, len(k.keys)),
	}
	for _, key := range k.keys {
		status := KeyStatus{
			Fingerprint: key.fingerprint,
			Masked:      key.masked,
			Used:        key.used,
			DailyLimit:  k.dailyLimit,
			Usable:      key.usable && key.used < k.dailyLimit,
			Reason:      key.reason,
		}
		out.Statuses = append(out.Statuses, status)
		switch {
		case status.Usable:
			out.Usable++
		case strings.Contains(status.Reason, "invalid"):
			out.Invalid++
		case strings.Contains(status.Reason, "quota") || strings.Contains(status.Reason, "limit"):
			out.Exhausted++
		default:
			out.Blocked++
		}
	}
	return out
}

func (k *KeyRing) Snapshot() []KeyStatus {
	return k.Summary().Statuses
}

func (k *KeyRing) UsableCount() int {
	return k.Summary().Usable
}

func (k *KeyRing) Do(ctx context.Context, client *http.Client, req *http.Request, userAgent string, requestCost int) (*http.Response, string, error) {
	if requestCost < 1 {
		requestCost = 1
	}
	maxAttempts := len(k.keys)
	if maxAttempts == 0 {
		return nil, "", ErrNoUsableAPIKey
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		idx, key, ok := k.acquire()
		if !ok {
			return nil, "", ErrNoUsableAPIKey
		}

		nextReq := req.Clone(ctx)
		nextReq.Header.Set("X-API-Key", key.raw)
		nextReq.Header.Set("Authorization", "Bearer "+key.raw)
		if strings.TrimSpace(userAgent) != "" {
			nextReq.Header.Set("User-Agent", userAgent)
		}

		resp, err := client.Do(nextReq)
		if err != nil {
			k.cooldown(idx, "network error", 8*time.Second)
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			k.invalidate(idx, "invalid key")
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if isQuotaLikeResponse(resp.StatusCode, body) {
				k.exhaust(idx, "quota exceeded")
			} else {
				k.cooldown(idx, "rate limited", 15*time.Second)
			}
			continue
		}
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if isQuotaLikeResponse(resp.StatusCode, body) {
				k.exhaust(idx, "quota exceeded")
				continue
			}
			return &http.Response{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Header:     resp.Header,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Request:    resp.Request,
			}, key.fingerprint, nil
		}
		day := currentDayUTC()
		usage, err := k.incrementUsage(key.fingerprint, day, requestCost)
		if err != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil, "", err
		}
		k.setUsed(idx, usage.Used)
		if usage.Used >= k.dailyLimit {
			k.exhaust(idx, "daily limit reached")
		} else {
			k.setState(idx, true, "ok")
		}
		return resp, key.fingerprint, nil
	}
	return nil, "", ErrNoUsableAPIKey
}

func (k *KeyRing) acquire() (int, keyState, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.keys) == 0 {
		return 0, keyState{}, false
	}
	now := time.Now().UTC()
	for i := 0; i < len(k.keys); i++ {
		idx := (k.next + i) % len(k.keys)
		key := k.keys[idx]
		if !key.usable {
			continue
		}
		if key.used >= k.dailyLimit {
			k.keys[idx].usable = false
			k.keys[idx].reason = "daily limit reached"
			continue
		}
		if !key.cooldownEnd.IsZero() && now.Before(key.cooldownEnd) {
			continue
		}
		k.next = (idx + 1) % len(k.keys)
		return idx, key, true
	}
	return 0, keyState{}, false
}

func (k *KeyRing) getKey(idx int) keyState {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keys[idx]
}

func (k *KeyRing) setUsed(idx int, used int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if idx < 0 || idx >= len(k.keys) {
		return
	}
	k.keys[idx].used = used
}

func (k *KeyRing) setState(idx int, usable bool, reason string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if idx < 0 || idx >= len(k.keys) {
		return
	}
	k.keys[idx].usable = usable
	k.keys[idx].reason = reason
	if usable {
		k.keys[idx].cooldownEnd = time.Time{}
	}
}

func (k *KeyRing) invalidate(idx int, reason string) {
	k.setState(idx, false, reason)
}

func (k *KeyRing) exhaust(idx int, reason string) {
	k.setState(idx, false, reason)
}

func (k *KeyRing) cooldown(idx int, reason string, duration time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if idx < 0 || idx >= len(k.keys) {
		return
	}
	k.keys[idx].reason = reason
	k.keys[idx].cooldownEnd = time.Now().UTC().Add(duration)
}

func (k *KeyRing) loadUsedForDay(fingerprint string, day string) (int, error) {
	if k.repo == nil {
		return 0, nil
	}
	usage, ok, err := k.repo.UsageForDay(fingerprint, day)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return usage.Used, nil
}

func (k *KeyRing) incrementUsage(fingerprint string, day string, delta int) (domain.APIKeyUsage, error) {
	if k.repo == nil {
		return domain.APIKeyUsage{Fingerprint: fingerprint, Day: day, Used: delta}, nil
	}
	return k.repo.IncrementUsage(fingerprint, day, delta)
}

func currentDayUTC() string {
	return time.Now().UTC().Format("2006-01-02")
}

func isQuotaLikeResponse(statusCode int, body []byte) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(string(body)))
	if text == "" {
		return false
	}
	quotaTokens := []string{"quota", "credit", "daily limit", "rate limit", "too many requests"}
	for _, token := range quotaTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func fingerprintKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])[:12]
}

func maskKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if len(trimmed) <= 8 {
		return "****"
	}
	return trimmed[:4] + "…" + trimmed[len(trimmed)-4:]
}
