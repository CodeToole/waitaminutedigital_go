package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvKeepsProcessEnvironmentAndReadsFileValues(t *testing.T) {
	t.Setenv("WAITAMINUTE_DOTENV_PRECEDENCE", "from-process")
	path := filepath.Join(t.TempDir(), ".env")
	contents := "# local config\nWAITAMINUTE_DOTENV_PRECEDENCE=from-file\nWAITAMINUTE_DOTENV_FILE='loaded value'\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("write test env file: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv() returned error: %v", err)
	}
	if got := os.Getenv("WAITAMINUTE_DOTENV_PRECEDENCE"); got != "from-process" {
		t.Errorf("existing process value = %q, want from-process", got)
	}
	if got := os.Getenv("WAITAMINUTE_DOTENV_FILE"); got != "loaded value" {
		t.Errorf("file value = %q, want loaded value", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("missing env file returned error: %v", err)
	}
}
