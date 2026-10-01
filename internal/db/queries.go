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
	articles, _, _, err := ListPublishedArticlesPage(ctx, database, category, 1, limit)
	return articles, err
}

func ListPublishedArticlesPage(ctx context.Context, database *sql.DB, category string, page int, perPage int) ([]models.Article, int, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	args := make([]any, 0, 2)
	countQuery := `SELECT COUNT(*) FROM article WHERE published = 1`
	if category != "" {
		countQuery += ` AND category = ?`
		args = append(args, category)
	}
	var total int
	if err := database.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, page, fmt.Errorf("count published articles: %w", err)
	}
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	query := `
		SELECT id, title, slug, category, summary, body_md, cover_image, published, created_at,
		       EXISTS(SELECT 1 FROM highlight WHERE article_id = article.id)
		FROM article
		WHERE published = 1`
	if category != "" {
		query += ` AND category = ?`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)
	articles, err := queryArticles(ctx, database, query, args...)
	if err != nil {
		return nil, 0, page, err
	}
	return articles, total, page, nil
}

func GetPublishedArticle(ctx context.Context, database *sql.DB, slug string) (models.Article, error) {
	const query = `
		SELECT id, title, slug, category, summary, body_md, cover_image, published, created_at,
		       EXISTS(SELECT 1 FROM highlight WHERE article_id = article.id)
		FROM article
		WHERE published = 1 AND slug = ?
		LIMIT 1`

	var article models.Article
	err := database.QueryRowContext(ctx, query, slug).Scan(
		&article.ID,
		&article.Title,
		&article.Slug,
		&article.Category,
		&article.Summary,
		&article.BodyMD,
		&article.CoverImage,
		&article.Published,
		&article.CreatedAt,
		&article.FeaturedHighlight,
	)
	if err != nil {
		return models.Article{}, fmt.Errorf("get published article: %w", err)
	}
	return article, nil
}

func CreateInquiry(ctx context.Context, database *sql.DB, inquiry models.Inquiry) error {
	const query = `
		INSERT INTO inquiry (name, email, subject, message)
		VALUES (?, ?, ?, ?)`
	if _, err := database.ExecContext(ctx, query, inquiry.Name, inquiry.Email, inquiry.Subject, inquiry.Message); err != nil {
		return fmt.Errorf("create inquiry: %w", err)
	}
	return nil
}

func queryArticles(ctx context.Context, database *sql.DB, query string, args ...any) ([]models.Article, error) {
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
			&article.FeaturedHighlight,
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
