package pokedata

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

type fakeResolver struct {
	id      string
	setName string
	setCode string
	err     error
}

func (f fakeResolver) ResolveCardID(_ context.Context, _ domain.Set, _ domain.Card) (string, string, string, error) {
	return f.id, f.setName, f.setCode, f.err
}

func TestRefreshCardUsesPriceProviderCardID(t *testing.T) {
	var gotID string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotID = req.URL.Query().Get("id")
			return jsonResponse(http.StatusOK, `[{"source":11,"avg":3.25},{"source":10,"avg":45.00}]`), nil
		}),
	}

	p := &Provider{
		client:   client,
		endpoint: "https://example.invalid/stats",
	}

	card := domain.Card{
		ID:                  "swsh3-136",
		PriceProviderCardID: "54321",
	}
	snapshot, err := p.RefreshCard(context.Background(), card, domain.Set{}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if gotID != "54321" {
		t.Fatalf("expected query id 54321, got %q", gotID)
	}
	if snapshot.Ungraded == nil || snapshot.Ungraded.Amount != 3.25 {
		t.Fatalf("expected ungraded 3.25, got %+v", snapshot.Ungraded)
	}
	if snapshot.PSA10 == nil || snapshot.PSA10.Amount != 45.0 {
		t.Fatalf("expected psa10 45.0, got %+v", snapshot.PSA10)
	}
}

func TestRefreshCardResolvesMissingProviderID(t *testing.T) {
	var gotID string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotID = req.URL.Query().Get("id")
			return jsonResponse(http.StatusOK, `[{"source":12,"avg":6.5},{"source":10,"avg":55.0}]`), nil
		}),
	}

	p := &Provider{
		client:   client,
		resolver: fakeResolver{id: "999", setName: "Some Set", setCode: "m2"},
		endpoint: "https://example.invalid/stats",
	}

	card := domain.Card{ID: "sv4a-349"}
	snapshot, err := p.RefreshCard(context.Background(), card, domain.Set{}, config.Default())
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if gotID != "999" {
		t.Fatalf("expected query id 999, got %q", gotID)
	}
	if snapshot.PriceProviderCardID != "999" {
		t.Fatalf("expected snapshot provider id 999, got %q", snapshot.PriceProviderCardID)
	}
	if snapshot.PriceProviderSetName != "Some Set" {
		t.Fatalf("expected snapshot set name Some Set, got %q", snapshot.PriceProviderSetName)
	}
	if snapshot.PriceProviderSetCode != "m2" {
		t.Fatalf("expected snapshot set code m2, got %q", snapshot.PriceProviderSetCode)
	}
}

func TestRefreshCardFailsWithoutProviderID(t *testing.T) {
	p := &Provider{
		client:   &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return jsonResponse(http.StatusOK, `[]`), nil })},
		resolver: fakeResolver{id: ""},
		endpoint: "http://127.0.0.1:1",
	}

	_, err := p.RefreshCard(context.Background(), domain.Card{ID: "sv4a-1"}, domain.Set{}, config.Default())
	if err == nil {
		t.Fatal("expected error when provider card id is unavailable")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "missing price provider card id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{URL: &url.URL{}},
	}
}
