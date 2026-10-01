package seed

import (
	"context"
	"database/sql"
	"fmt"
)

func Apply(ctx context.Context, database *sql.DB) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer transaction.Rollback()

	const articleQuery = `
		INSERT INTO article (title, slug, category, summary, body_md, cover_image, published)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title = excluded.title,
			category = excluded.category,
			summary = excluded.summary,
			body_md = excluded.body_md,
			cover_image = excluded.cover_image,
			published = excluded.published`
	if _, err := transaction.ExecContext(ctx, articleQuery,
		"Building an Expository Teaching Engine: Bible Study App",
		"building-an-expository-teaching-engine-bible-study-app",
		"Devlog",
		"An offline-first Flutter study tool engineered for deep expository scripture study, automatic lesson outline parsing, interactive Podium Mode, and classroom syllabus PDF generation.",
		"## Full write-up coming soon\n\nA practical study tool built for deep expository scripture study.",
		"/static/img/mascot_head.webp",
		true,
	); err != nil {
		return fmt.Errorf("seed Bible Study article: %w", err)
	}

	const highlightQuery = `
		INSERT INTO highlight (id, title, kicker, summary, href, image, sort_order, published)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			kicker = excluded.kicker,
			summary = excluded.summary,
			href = excluded.href,
			image = excluded.image,
			sort_order = excluded.sort_order,
			published = excluded.published`
	highlights := []struct {
		id        int
		title     string
		kicker    string
		summary   string
		href      string
		image     string
		sortOrder int
	}{
		{1, "Bible Study App", "Project", "An offline-first tool for focused expository study.", "/projects", "/static/img/logo-128.webp", 1},
		{2, "Game Room", "Coming Soon", "The first playable build is taking shape.", "/game-room", "/static/img/mascot_head.webp", 2},
	}
	for _, highlight := range highlights {
		if _, err := transaction.ExecContext(ctx, highlightQuery,
			highlight.id,
			highlight.title,
			highlight.kicker,
			highlight.summary,
			highlight.href,
			highlight.image,
			highlight.sortOrder,
			true,
		); err != nil {
			return fmt.Errorf("seed %q highlight: %w", highlight.title, err)
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}
