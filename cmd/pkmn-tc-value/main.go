package main

import (
	"context"
	"log"

	"github.com/Official-Husko/pkmn-tc-value/internal/app"
)

// TODO: add support to export collection as a csv, json and html file
// TODO: more colors and animations in the terminal output
// TODO: show more data for the cards in the detail view
// TODO: downloading card images has the amount twice printed
// TODO: show the collected amount of cards and total amount  next to name in list
// TODO: display some set stats and infos when selecting one at the top,
// e.g. total cards, total value, collected ones, hidden ones and normal amount, release date etc.
// maybe even the logo of the set
// TODO: add support for multiple variants of 1 card, e.g. holo, reverse holo, full art, secret rare etc. and show them in the detail view
// TODO: maybe a selection if more than 1 variant exists in the list view, e.g. "Pikachu (3 variants)"
// TODO: show some stats on start, e.g. total value of collection, total amount of cards, most valuable card etc.
// TODO: Directly allow searching for new cards when looking at a card
// TODO: allow looking into the collection, same behavior as for new searches, but with the collection as source

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
