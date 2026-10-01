package config

import (
	"os"
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
