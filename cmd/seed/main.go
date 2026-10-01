package main

import (
	"context"
	"log"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/seed"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load configuration: %v", err)
	}
	database, err := db.Open(cfg.SiteDB)
	if err != nil {
		log.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	if err := seed.Apply(context.Background(), database); err != nil {
		log.Fatalf("Seed database: %v", err)
	}
	log.Print("Seeded 1 article and 2 highlights")
}
