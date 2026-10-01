package main

import (
	"log"
	"os"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
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
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("Create upload directory: %v", err)
	}

	addr := ":" + cfg.Port
	log.Printf("Starting Waitaminute Digital server on %s", addr)
	if err := newServer(cfg.SiteURL, database, serverOptions{
		AdminPasswordHash: cfg.AdminPasswordHash,
		Production:        cfg.Production,
		UploadDir:         cfg.UploadDir,
	}).Start(addr); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
