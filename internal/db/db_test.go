package db

import (
	"path/filepath"
	"testing"
)

func TestOpenMigratesAndEnforcesUniqueArticleSlug(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatalf("Open() returned unexpected error: %v", err)
	}
	defer database.Close()

	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second Migrate() returned unexpected error: %v", err)
	}

	const insert = `INSERT INTO article (title, slug, category) VALUES (?, ?, ?)`
	if _, err := database.Exec(insert, "First", "same-slug", "Devlog"); err != nil {
		t.Fatalf("insert first article: %v", err)
	}
	if _, err := database.Exec(insert, "Duplicate", "same-slug", "News"); err == nil {
		t.Fatal("duplicate article slug was accepted")
	}
}
