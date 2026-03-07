package main

import (
	"context"
	"log"

	"github.com/Official-Husko/pkmn-tc-value/internal/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
