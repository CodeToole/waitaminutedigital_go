package config

import (
	"fmt"
	"os"
	"runtime"
)

// Config holds all runtime configuration settings for the web application.
// In Go, structs group related fields together (similar to Python dataclasses or classes with attributes).
type Config struct {
	Port              string
	SiteDB            string
	SiteURL           string
	AdminPasswordHash string
	SessionSecret     string
}

// Load reads configuration from environment variables, supplying safe local defaults.
func Load() (Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://waitaminutedigital.com"
	}

	siteDB := os.Getenv("SITE_DB")
	if siteDB == "" {
		if runtime.GOOS == "windows" {
			siteDB = "./data/site.db"
			if err := os.MkdirAll("data", 0755); err != nil {
				return Config{}, fmt.Errorf("create database directory: %w", err)
			}
		} else {
			siteDB = "/home/data/site.db"
		}
	}

	return Config{
		Port:              port,
		SiteDB:            siteDB,
		SiteURL:           siteURL,
		AdminPasswordHash: os.Getenv("ADMIN_PASSWORD_HASH"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
	}, nil
}
