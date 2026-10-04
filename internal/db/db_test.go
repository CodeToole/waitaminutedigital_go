package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/CodeToole/waitaminutedigital_go/migrations"
)

func TestFreshDatabaseAppliesMigrationsAndRepeatedMigrationIsNoOp(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatalf("Open() returned unexpected error: %v", err)
	}
	defer database.Close()

	before := readMigrationRecords(t, database)
	if len(before) != 2 || before[0].version != 1 || before[1].version != 2 {
		t.Fatalf("migration versions = %v, want [1 2]", migrationVersions(before))
	}
	for _, record := range before {
		if record.appliedAt == "" {
			t.Errorf("migration %d has empty applied_at", record.version)
		}
	}

	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second Migrate() returned unexpected error: %v", err)
	}
	after := readMigrationRecords(t, database)
	if len(after) != len(before) {
		t.Fatalf("migration record count after second run = %d, want %d", len(after), len(before))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Errorf("migration record %d changed from %+v to %+v", before[i].version, before[i], after[i])
		}
	}
}

func TestOpenBaselinesLegacyDatabaseWithoutDataLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	legacy.SetMaxOpenConns(1)
	if _, err := legacy.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	for _, name := range []string{"001_init.sql", "002_highlight_article.sql"} {
		script, err := migrations.Files.ReadFile(name)
		if err != nil {
			t.Fatalf("read legacy migration %s: %v", name, err)
		}
		if _, err := legacy.Exec(string(script)); err != nil {
			t.Fatalf("apply legacy migration %s: %v", name, err)
		}
	}
	if _, err := legacy.Exec(`INSERT INTO article (id, title, slug, category) VALUES (41, 'Existing article', 'existing-article', 'Devlog')`); err != nil {
		t.Fatalf("insert legacy article: %v", err)
	}
	if _, err := legacy.Exec(`INSERT INTO highlight (id, title, article_id) VALUES (7, 'Existing highlight', 41)`); err != nil {
		t.Fatalf("insert legacy highlight: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open() legacy database: %v", err)
	}
	defer database.Close()

	var title, slug string
	if err := database.QueryRow(`SELECT title, slug FROM article WHERE id = 41`).Scan(&title, &slug); err != nil {
		t.Fatalf("read existing article: %v", err)
	}
	if title != "Existing article" || slug != "existing-article" {
		t.Errorf("existing article = (%q, %q), want (%q, %q)", title, slug, "Existing article", "existing-article")
	}
	var highlightTitle string
	var articleID int
	if err := database.QueryRow(`SELECT title, article_id FROM highlight WHERE id = 7`).Scan(&highlightTitle, &articleID); err != nil {
		t.Fatalf("read existing highlight: %v", err)
	}
	if highlightTitle != "Existing highlight" || articleID != 41 {
		t.Errorf("existing highlight = (%q, %d), want (%q, %d)", highlightTitle, articleID, "Existing highlight", 41)
	}
	records := readMigrationRecords(t, database)
	if got := migrationVersions(records); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("baselined migration versions = %v, want [1 2]", got)
	}
}

func TestOpenMigratesAndEnforcesUniqueArticleSlug(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatalf("Open() returned unexpected error: %v", err)
	}
	defer database.Close()

	const insert = `INSERT INTO article (title, slug, category) VALUES (?, ?, ?)`
	if _, err := database.Exec(insert, "First", "same-slug", "Devlog"); err != nil {
		t.Fatalf("insert first article: %v", err)
	}
	if _, err := database.Exec(insert, "Duplicate", "same-slug", "News"); err == nil {
		t.Fatal("duplicate article slug was accepted")
	}
}

type migrationRecord struct {
	version   int
	appliedAt string
}

func readMigrationRecords(t *testing.T, database *sql.DB) []migrationRecord {
	t.Helper()
	rows, err := database.Query(`SELECT version, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("read migration records: %v", err)
	}
	defer rows.Close()

	var records []migrationRecord
	for rows.Next() {
		var record migrationRecord
		if err := rows.Scan(&record.version, &record.appliedAt); err != nil {
			t.Fatalf("scan migration record: %v", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration records: %v", err)
	}
	return records
}

func migrationVersions(records []migrationRecord) []int {
	versions := make([]int, len(records))
	for i, record := range records {
		versions[i] = record.version
	}
	return versions
}
