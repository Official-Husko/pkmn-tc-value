package syncer

import (
	"testing"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestNeedsRefresh(t *testing.T) {
	svc := &CardRefreshService{}
	cfg := config.Default()

	if !svc.NeedsRefresh(domain.Card{}, cfg) {
		t.Fatal("expected refresh when no previous check exists")
	}

	fresh := time.Now().Add(-2 * time.Hour)
	if svc.NeedsRefresh(domain.Card{PriceCheckedAt: &fresh}, cfg) {
		t.Fatal("did not expect refresh for fresh card")
	}

	stale := time.Now().Add(-72 * time.Hour)
	if !svc.NeedsRefresh(domain.Card{PriceCheckedAt: &stale}, cfg) {
		t.Fatal("expected refresh for stale card")
	}
}
