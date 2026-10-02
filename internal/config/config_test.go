package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadWindowsDefaultSiteDBCreatesDataDirectory(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("Windows-specific default")
	}

	t.Chdir(t.TempDir())
	t.Setenv("SITE_DB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.SiteDB != "./data/site.db" {
		t.Fatalf("SiteDB = %q, want %q", cfg.SiteDB, "./data/site.db")
	}
	if info, err := os.Stat("data"); err != nil || !info.IsDir() {
		t.Fatalf("data directory was not created: info=%v, err=%v", info, err)
	}
}

func TestProductionRequiresAdminHashAndSessionSecret(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "production")
	t.Setenv("SITE_DB", "./data/site.db")
	t.Setenv("ADMIN_PASSWORD_HASH", "")
	t.Setenv("SESSION_SECRET", "")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "ADMIN_PASSWORD_HASH") {
		t.Fatalf("Load() error = %v, want missing ADMIN_PASSWORD_HASH", err)
	}

	t.Setenv("ADMIN_PASSWORD_HASH", "bcrypt-hash")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("Load() error = %v, want missing SESSION_SECRET", err)
	}

	t.Setenv("SESSION_SECRET", "too-short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "at least 32 bytes") {
		t.Fatalf("Load() error = %v, want minimum SESSION_SECRET length", err)
	}

	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	if _, err := Load(); err != nil {
		t.Fatalf("Load() with required production secrets returned error: %v", err)
	}
}
