package catalog

import (
	"context"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

type Provider interface {
	Name() string
	FetchSets(ctx context.Context) ([]domain.RemoteSet, error)
	FetchCardsForSet(ctx context.Context, setID string) ([]domain.RemoteCard, error)
}
