package config

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var clarityIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

const defaultNotifyTo = "corneliustoole@waitaminutedigital.com"

// Config holds all runtime configuration settings for the web application.
// In Go, structs group related fields together (similar to Python dataclasses or classes with attributes).
type Config struct {
	Port              string
	SiteDB            string
	SiteURL           string
	AdminPasswordHash string
	SessionSecret     string
	Production        bool
	UploadDir         string
	ClarityID         string
	ACSEndpoint       string
	ACSAccessKey      string
	NotifyFrom        string
	NotifyTo          string
	CSPEnforce        bool
}

// Load reads configuration from environment variables, supplying safe local defaults.
func Load() (Config, error) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if err := loadDotEnv(".env"); err != nil {
			return Config{}, fmt.Errorf("load local environment: %w", err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://waitaminutedigital.com"
	}
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	adminPasswordHash := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH"))
	sessionSecret := os.Getenv("SESSION_SECRET")
	if production {
		if adminPasswordHash == "" {
			return Config{}, fmt.Errorf("ADMIN_PASSWORD_HASH is required in production")
		}
		if len(sessionSecret) < 32 {
			return Config{}, fmt.Errorf("SESSION_SECRET must be at least 32 bytes in production")
		}
	}
	uploadDir := "./data/uploads"
	if production {
		uploadDir = "/home/data/uploads"
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

	clarityID := strings.TrimSpace(os.Getenv("CLARITY_ID"))
	if clarityID != "" && !clarityIDPattern.MatchString(clarityID) {
		log.Printf("CLARITY_ID %q is not alphanumeric; ignoring it", clarityID)
		clarityID = ""
	}

	notifyTo := strings.TrimSpace(os.Getenv("NOTIFY_TO"))
	if notifyTo == "" {
		notifyTo = defaultNotifyTo
	}
	cspEnforce := strings.EqualFold(strings.TrimSpace(os.Getenv("CSP_ENFORCE")), "true")

	return Config{
		Port:              port,
		SiteDB:            siteDB,
		SiteURL:           siteURL,
		AdminPasswordHash: adminPasswordHash,
		SessionSecret:     sessionSecret,
		Production:        production,
		UploadDir:         uploadDir,
		ClarityID:         clarityID,
		ACSEndpoint:       strings.TrimSpace(os.Getenv("ACS_ENDPOINT")),
		ACSAccessKey:      strings.TrimSpace(os.Getenv("ACS_ACCESS_KEY")),
		NotifyFrom:        strings.TrimSpace(os.Getenv("NOTIFY_FROM")),
		NotifyTo:          notifyTo,
		CSPEnforce:        cspEnforce,
	}, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("invalid .env entry on line %d", lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from .env: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .env: %w", err)
	}
	return nil
}
