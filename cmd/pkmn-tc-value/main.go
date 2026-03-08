package main

import (
	"context"
	"log"

	"github.com/Official-Husko/pkmn-tc-value/internal/app"
)

// fix bad image quality in card renderer
// add support to export collection as a csv, json and html file
// save user cards to collection.db file
// more colors and animations in the terminal output
// show more data for the cards in the detail view
// downloading card images has the amount twice printed
// show the collected amount of cards and total amount  next to name in list
// display some set stats and infos when selecting one at the top,
// e.g. total cards, total value, collected ones, hidden ones and normal amount, release date etc.
// maybe even the logo of the set

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
