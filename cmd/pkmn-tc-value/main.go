package main

import (
	"context"
	"log"

	"github.com/Official-Husko/pkmn-tc-value/internal/app"
)

// fix bad image quality in card renderer
// add support to export collection as a csv, json and html file
// multiple workers downloading card images in parallel, configurable by the user in config file
// save user cards to collection.db file

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
