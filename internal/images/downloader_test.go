package images

import (
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestImageURLCandidatesTemplateFirstThenFallback(t *testing.T) {
	card := domain.Card{
		ImageBaseURL: "https://assets.tcgdex.net/en/sv/sv4a/95",
		SetCode:      "sv4a",
		SetName:      "Wild Force",
		Number:       "095",
		Language:     "English",
		ImageURL:     "https://example.invalid/from-json.webp",
	}

	got := imageURLCandidates(card, true)
	if len(got) != 4 {
		t.Fatalf("expected 4 candidates, got %d: %#v", len(got), got)
	}
	if got[0] != "https://assets.tcgdex.net/en/sv/sv4a/95/high.png" {
		t.Fatalf("unexpected first candidate: %q", got[0])
	}
	if got[1] != "https://images.scrydex.com/pokemon/sv4a-095/large" {
		t.Fatalf("unexpected second candidate: %q", got[1])
	}
	if got[2] != "https://pokemoncardimages.pokedata.io/images/Wild+Force/095.webp" {
		t.Fatalf("unexpected third candidate: %q", got[2])
	}
	if got[3] != card.ImageURL {
		t.Fatalf("unexpected fallback candidate: %q", got[3])
	}
}

func TestImageURLCandidatesNoBackup(t *testing.T) {
	card := domain.Card{
		ImageBaseURL: "https://assets.tcgdex.net/en/sv/sv4a/95",
		SetCode:      "sv4a",
		SetName:      "Wild Force",
		Number:       "095",
		Language:     "English",
		ImageURL:     "https://example.invalid/from-json.webp",
	}

	got := imageURLCandidates(card, false)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(got), got)
	}
	if got[0] != "https://assets.tcgdex.net/en/sv/sv4a/95/high.png" {
		t.Fatalf("unexpected primary candidate: %q", got[0])
	}
}

func TestImageURLCandidatesNoBackupNoScrydexCandidate(t *testing.T) {
	card := domain.Card{
		ImageBaseURL: "",
		SetCode:      "",
		SetName:      "Wild Force",
		Number:       "095",
		Language:     "English",
		ImageURL:     "https://pokemoncardimages.pokedata.io/images/Wild+Force/095.webp",
	}

	got := imageURLCandidates(card, false)
	if len(got) != 0 {
		t.Fatalf("expected 0 candidates when backup is disabled and tcgdex URL is missing, got %d: %#v", len(got), got)
	}
}

func TestTcgdexImageURL(t *testing.T) {
	card := domain.Card{ImageBaseURL: "https://assets.tcgdex.net/en/swsh/swsh3/136"}
	got := tcgdexImageURL(card)
	want := "https://assets.tcgdex.net/en/swsh/swsh3/136/high.png"
	if got != want {
		t.Fatalf("unexpected URL\nwant: %s\ngot:  %s", want, got)
	}
}

func TestScrydexImageURLJapanese(t *testing.T) {
	card := domain.Card{
		SetCode:  "sv4a",
		Number:   "349",
		Language: "Japanese",
	}
	got := scrydexImageURL(card)
	want := "https://images.scrydex.com/pokemon/sv4a_ja-349/large"
	if got != want {
		t.Fatalf("unexpected URL\nwant: %s\ngot:  %s", want, got)
	}
}

func TestScrydexImageURLJapaneseNoDoubleSuffix(t *testing.T) {
	card := domain.Card{
		SetCode:  "sv4a_ja",
		Number:   "349",
		Language: "Japanese",
	}
	got := scrydexImageURL(card)
	want := "https://images.scrydex.com/pokemon/sv4a_ja-349/large"
	if got != want {
		t.Fatalf("unexpected URL\nwant: %s\ngot:  %s", want, got)
	}
}
