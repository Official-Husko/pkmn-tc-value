package pokemonpricetracker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

type memoryUsageRepo struct {
	data map[string]domain.APIKeyUsage
}

func (m *memoryUsageRepo) UsageForDay(fingerprint string, day string) (domain.APIKeyUsage, bool, error) {
	usage, ok := m.data[fingerprint]
	if !ok || usage.Day != day {
		return domain.APIKeyUsage{}, false, nil
	}
	return usage, true, nil
}

func (m *memoryUsageRepo) IncrementUsage(fingerprint string, day string, delta int) (domain.APIKeyUsage, error) {
	usage := m.data[fingerprint]
	if usage.Day != day {
		usage = domain.APIKeyUsage{
			Fingerprint: fingerprint,
			Day:         day,
		}
	}
	usage.Used += delta
	m.data[fingerprint] = usage
	return usage, nil
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidateFiltersKeyStates(t *testing.T) {
	repo := &memoryUsageRepo{data: make(map[string]domain.APIKeyUsage)}
	ring := NewKeyRing([]string{"good-key", "bad-key", "quota-key"}, 100, repo)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			auth := req.Header.Get("Authorization")
			switch {
			case strings.Contains(auth, "good-key"):
				return testResponse(http.StatusOK, `{"success":true}`), nil
			case strings.Contains(auth, "bad-key"):
				return testResponse(http.StatusUnauthorized, `{"success":false}`), nil
			case strings.Contains(auth, "quota-key"):
				return testResponse(http.StatusTooManyRequests, `{"error":"quota exceeded"}`), nil
			default:
				return testResponse(http.StatusInternalServerError, `{}`), nil
			}
		}),
	}

	summary, err := ring.Validate(context.Background(), client, "test-agent")
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if summary.Total != 3 {
		t.Fatalf("expected total keys=3, got %d", summary.Total)
	}
	if summary.Usable != 1 {
		t.Fatalf("expected usable keys=1, got %d", summary.Usable)
	}
	if summary.Invalid != 1 {
		t.Fatalf("expected invalid keys=1, got %d", summary.Invalid)
	}
	if summary.Exhausted != 1 {
		t.Fatalf("expected exhausted keys=1, got %d", summary.Exhausted)
	}
}

func TestDoRotatesToNextUsableKey(t *testing.T) {
	repo := &memoryUsageRepo{data: make(map[string]domain.APIKeyUsage)}
	ring := NewKeyRing([]string{"bad-key", "good-key"}, 100, repo)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			auth := req.Header.Get("Authorization")
			if strings.Contains(auth, "bad-key") {
				return testResponse(http.StatusUnauthorized, `{"error":"invalid key"}`), nil
			}
			return testResponse(http.StatusOK, `{"success":true}`), nil
		}),
	}
	if _, err := ring.Validate(context.Background(), client, "test-agent"); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
	resp, _, err := ring.Do(context.Background(), client, req, "test-agent", 1)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{},
	}
}
