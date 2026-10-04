package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/notify"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Web server: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := views.InitializeAssetVersion("static"); err != nil {
		return fmt.Errorf("initialize static asset version: %w", err)
	}
	database, err := db.Open(cfg.SiteDB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Close database: %v", err)
		}
	}()
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}

	var notifier notify.Notifier = notify.NoopNotifier{}
	if cfg.ACSEndpoint != "" && cfg.ACSAccessKey != "" && cfg.NotifyFrom != "" {
		notifier = notify.NewACSNotifier(cfg.ACSEndpoint, cfg.ACSAccessKey, cfg.NotifyFrom, cfg.NotifyTo)
	} else {
		log.Print("Inquiry email notifications disabled: set ACS_ENDPOINT, ACS_ACCESS_KEY, and NOTIFY_FROM to enable them")
	}

	addr := ":" + cfg.Port
	log.Printf("Starting Waitaminute Digital server on %s", addr)
	server := newHTTPServer(addr, newServer(cfg.SiteURL, database, serverOptions{
		AdminPasswordHash: cfg.AdminPasswordHash,
		Production:        cfg.Production,
		UploadDir:         cfg.UploadDir,
		ClarityID:         cfg.ClarityID,
		Notifier:          notifier,
		SessionSecret:     cfg.SessionSecret,
		CSPEnforce:        cfg.CSPEnforce,
	}))

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server stopped: %w", err)
	case <-signalContext.Done():
		log.Print("Shutdown signal received; stopping server")
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := server.Shutdown(shutdownContext)
		cancel()
		if shutdownErr != nil {
			if err := server.Close(); err != nil {
				log.Printf("Force close server: %v", err)
			}
		}
		serveErr := <-serverErrors
		log.Print("Waitaminute Digital server stopped")
		if shutdownErr != nil {
			return fmt.Errorf("graceful server shutdown: %w", shutdownErr)
		}
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("server stopped during shutdown: %w", serveErr)
		}
		return nil
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}
}
