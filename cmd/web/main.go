package main

import (
	"log"
	"os"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/notify"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load configuration: %v", err)
	}
	if err := views.InitializeAssetVersion("static"); err != nil {
		log.Fatalf("Initialize static asset version: %v", err)
	}
	database, err := db.Open(cfg.SiteDB)
	if err != nil {
		log.Fatalf("Open database: %v", err)
	}
	defer database.Close()
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("Create upload directory: %v", err)
	}

	var notifier notify.Notifier = notify.NoopNotifier{}
	if cfg.ACSEndpoint != "" && cfg.ACSAccessKey != "" && cfg.NotifyFrom != "" {
		notifier = notify.NewACSNotifier(cfg.ACSEndpoint, cfg.ACSAccessKey, cfg.NotifyFrom, cfg.NotifyTo)
	} else {
		log.Print("Inquiry email notifications disabled: set ACS_ENDPOINT, ACS_ACCESS_KEY, and NOTIFY_FROM to enable them")
	}

	addr := ":" + cfg.Port
	log.Printf("Starting Waitaminute Digital server on %s", addr)
	if err := newServer(cfg.SiteURL, database, serverOptions{
		AdminPasswordHash: cfg.AdminPasswordHash,
		Production:        cfg.Production,
		UploadDir:         cfg.UploadDir,
		ClarityID:         cfg.ClarityID,
		Notifier:          notifier,
		SessionSecret:     cfg.SessionSecret,
	}).Start(addr); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
