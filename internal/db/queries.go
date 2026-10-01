package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/CodeToole/waitaminutedigital_go/internal/models"
)

func ListPublishedHighlights(ctx context.Context, database *sql.DB) ([]models.Highlight, error) {
	const query = `
		SELECT id, title, kicker, summary, href, image, sort_order, published
		FROM highlight
		WHERE published = 1
		ORDER BY sort_order, id`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query published highlights: %w", err)
	}
	defer rows.Close()

	highlights := make([]models.Highlight, 0)
	for rows.Next() {
		var highlight models.Highlight
		if err := rows.Scan(
			&highlight.ID,
			&highlight.Title,
			&highlight.Kicker,
			&highlight.Summary,
			&highlight.Href,
			&highlight.Image,
			&highlight.SortOrder,
			&highlight.Published,
		); err != nil {
			return nil, fmt.Errorf("scan published highlight: %w", err)
		}
		highlights = append(highlights, highlight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published highlights: %w", err)
	}
	return highlights, nil
}

func ListPublishedArticles(ctx context.Context, database *sql.DB, category string, limit int) ([]models.Article, error) {
	if limit < 1 || limit > 100 {
		limit = 6
	}

	query := `
		SELECT id, title, slug, category, summary, body_md, cover_image, published, created_at
		FROM article
		WHERE published = 1`
	args := make([]any, 0, 2)
	if category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query published articles: %w", err)
	}
	defer rows.Close()

	articles := make([]models.Article, 0)
	for rows.Next() {
		var article models.Article
		if err := rows.Scan(
			&article.ID,
			&article.Title,
			&article.Slug,
			&article.Category,
			&article.Summary,
			&article.BodyMD,
			&article.CoverImage,
			&article.Published,
			&article.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan published article: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published articles: %w", err)
	}
	return articles, nil
}
