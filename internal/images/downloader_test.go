package images

import (
	"testing"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func TestImageURLCandidatesPrimaryThenFallback(t *testing.T) {
	card := domain.Card{
		PriceProviderCardID: "ed3941fd9d7b78ff2bfb8ac788440743e69798cb3ca3883537049b456284bf2edf978178c8f5532c15bd98f25d1edd",
		TCGPlayerID:         "577335",
		ImageBaseURL:        "https://assets.tcgdex.net/en/sv/sv4a/95",
		SetCode:             "sv4a",
		SetName:             "Wild Force",
		Number:              "095",
		Language:            "English",
		ImageURL:            "https://example.invalid/from-json.webp",
	}

	got := imageURLCandidates(card, true)
	if len(got) != 4 {
		t.Fatalf("expected 4 candidates, got %d: %#v", len(got), got)
	}
	if got[0] != "https://assets.tcgdex.net/en/sv/sv4a/95/high.png" {
		t.Fatalf("unexpected first candidate: %q", got[0])
	}
	if got[1] != "https://api.pokewallet.io/images/pk_ed3941fd9d7b78ff2bfb8ac788440743e69798cb3ca3883537049b456284bf2edf978178c8f5532c15bd98f25d1edd?size=high" {
		t.Fatalf("unexpected second candidate: %q", got[1])
	}
	if got[2] != "https://images.scrydex.com/pokemon/sv4a-095/large" {
		t.Fatalf("unexpected third candidate: %q", got[2])
	}
	if got[3] != card.ImageURL {
		t.Fatalf("unexpected fourth candidate: %q", got[3])
	}
}

func TestImageURLCandidatesNoBackup(t *testing.T) {
	card := domain.Card{
		PriceProviderCardID: "ed3941fd9d7b78ff2bfb8ac788440743e69798cb3ca3883537049b456284bf2edf978178c8f5532c15bd98f25d1edd",
		TCGPlayerID:         "577335",
		ImageBaseURL:        "https://assets.tcgdex.net/en/sv/sv4a/95",
		SetCode:             "sv4a",
		SetName:             "Wild Force",
		Number:              "095",
		Language:            "English",
		ImageURL:            "https://example.invalid/from-json.webp",
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
		ImageURL:     "https://example.invalid/from-json.webp",
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

func TestPokewalletImageURLFromStrippedProviderID(t *testing.T) {
	card := domain.Card{
		PriceProviderCardID: "6ce5973b5944bf579404099fea3d8a594e4d4a65020f2429b8cb4f1b2c62e6d9e69612c57c0f7309296e7040ab8a",
		TCGPlayerID:         "577294",
	}
	got := pokewalletImageURL(card)
	want := "https://api.pokewallet.io/images/pk_6ce5973b5944bf579404099fea3d8a594e4d4a65020f2429b8cb4f1b2c62e6d9e69612c57c0f7309296e7040ab8a?size=high"
	if got != want {
		t.Fatalf("unexpected URL\nwant: %s\ngot:  %s", want, got)
	}
}
