package pokedata

import (
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestMatchRemoteCards(t *testing.T) {
	remote := []domain.RemoteCard{
		{ID: "sv4a-095", Number: "095", Name: "Pikachu"},
		{ID: "sv4a-096", Number: "096", Name: "Raichu"},
		{ID: "sv4a-097", Number: "097", Name: "Eevee"},
	}
	priceCards := []PokeCard{
		{ID: "1001", Number: "095", Name: "Pikachu"},
		{ID: "1002", Number: "096", Name: "Raichu"},
		{ID: "1003", Number: "097", Name: "Different Name"},
	}

	got := MatchRemoteCards(remote, priceCards)
	if got["sv4a-095"] != "1001" {
		t.Fatalf("expected sv4a-095 -> 1001, got %q", got["sv4a-095"])
	}
	if got["sv4a-096"] != "1002" {
		t.Fatalf("expected sv4a-096 -> 1002, got %q", got["sv4a-096"])
	}
	if got["sv4a-097"] != "1003" {
		t.Fatalf("expected sv4a-097 to use unique number fallback, got %q", got["sv4a-097"])
	}
}

func TestMatchRemoteCardsAmbiguousFallsBackDeterministic(t *testing.T) {
	remote := []domain.RemoteCard{
		{ID: "sv4a-095", Number: "095", Name: "Pikachu"},
	}
	priceCards := []PokeCard{
		{ID: "1001", Number: "095", Name: "Alt Pikachu"},
		{ID: "1002", Number: "095", Name: "Another Pikachu"},
	}

	got := MatchRemoteCards(remote, priceCards)
	if got["sv4a-095"] != "1001" {
		t.Fatalf("expected deterministic fallback match 1001, got %q", got["sv4a-095"])
	}
}

func TestMatchLocalCard(t *testing.T) {
	card := domain.Card{Number: "136", Name: "Furret"}
	priceCards := []PokeCard{
		{ID: "222", Number: "136", Name: "Furret"},
	}
	if got := MatchLocalCard(card, priceCards); got != "222" {
		t.Fatalf("expected 222, got %q", got)
	}
}

func TestMatchLocalCardAmbiguousFallsBackDeterministic(t *testing.T) {
	card := domain.Card{Number: "095", Name: "Pikachu"}
	priceCards := []PokeCard{
		{ID: "1002", Number: "095", Name: "Alt Pikachu"},
		{ID: "1001", Number: "095", Name: "Another Pikachu"},
	}
	if got := MatchLocalCard(card, priceCards); got != "1001" {
		t.Fatalf("expected deterministic fallback 1001, got %q", got)
	}
}
