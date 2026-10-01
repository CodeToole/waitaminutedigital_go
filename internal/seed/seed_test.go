package seed

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/CodeToole/waitaminutedigital_go/internal/db"
)

func TestApplyTwicePreservesCountsAndArticleCreatedAt(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close()

	if err := Apply(context.Background(), database); err != nil {
		t.Fatalf("first Apply() returned error: %v", err)
	}
	articleCount, highlightCount, createdAt := seedState(t, database)
	if articleCount != 1 || highlightCount != 2 {
		t.Fatalf("first run counts = %d article(s), %d highlight(s), want 1 and 2", articleCount, highlightCount)
	}

	if err := Apply(context.Background(), database); err != nil {
		t.Fatalf("second Apply() returned error: %v", err)
	}
	afterArticleCount, afterHighlightCount, afterCreatedAt := seedState(t, database)
	if afterArticleCount != articleCount || afterHighlightCount != highlightCount {
		t.Fatalf("counts changed after rerun: articles %d -> %d, highlights %d -> %d", articleCount, afterArticleCount, highlightCount, afterHighlightCount)
	}
	if afterCreatedAt != createdAt {
		t.Fatalf("created_at changed from %q to %q", createdAt, afterCreatedAt)
	}
}

func seedState(t *testing.T, database *sql.DB) (int, int, string) {
	t.Helper()
	var articleCount, highlightCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM article`).Scan(&articleCount); err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM highlight`).Scan(&highlightCount); err != nil {
		t.Fatalf("count highlights: %v", err)
	}
	const slug = "building-an-expository-teaching-engine-bible-study-app"
	var createdAt string
	if err := database.QueryRow(`SELECT created_at FROM article WHERE slug = ?`, slug).Scan(&createdAt); err != nil {
		t.Fatalf("read created_at: %v", err)
	}
	return articleCount, highlightCount, createdAt
}
