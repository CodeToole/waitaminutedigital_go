package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/CodeToole/waitaminutedigital_go/internal/models"
)

func ListAdminArticles(ctx context.Context, database *sql.DB) ([]models.Article, error) {
	const query = `
		SELECT id, title, slug, category, summary, body_md, cover_image, published, created_at,
		       EXISTS(SELECT 1 FROM highlight WHERE article_id = article.id)
		FROM article
		ORDER BY created_at DESC, id DESC`
	return queryArticles(ctx, database, query)
}

func GetArticle(ctx context.Context, database *sql.DB, id int64) (models.Article, error) {
	const query = `
		SELECT id, title, slug, category, summary, body_md, cover_image, published, created_at,
		       EXISTS(SELECT 1 FROM highlight WHERE article_id = article.id)
		FROM article
		WHERE id = ?`
	var article models.Article
	err := database.QueryRowContext(ctx, query, id).Scan(
		&article.ID, &article.Title, &article.Slug, &article.Category, &article.Summary,
		&article.BodyMD, &article.CoverImage, &article.Published, &article.CreatedAt, &article.FeaturedHighlight,
	)
	if err != nil {
		return models.Article{}, fmt.Errorf("get article: %w", err)
	}
	return article, nil
}

func ArticleSlugExists(ctx context.Context, database *sql.DB, slug string, exceptID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM article WHERE slug = ?)`
	args := []any{slug}
	if exceptID > 0 {
		query = `SELECT EXISTS(SELECT 1 FROM article WHERE slug = ? AND id != ?)`
		args = append(args, exceptID)
	}
	var exists bool
	if err := database.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check article slug: %w", err)
	}
	return exists, nil
}

func CreateArticle(ctx context.Context, database *sql.DB, article models.Article) (int64, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin create article: %w", err)
	}
	defer tx.Rollback()
	const query = `
		INSERT INTO article (title, slug, category, summary, body_md, cover_image, published)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, query, article.Title, article.Slug, article.Category,
		article.Summary, article.BodyMD, article.CoverImage, article.Published)
	if err != nil {
		return 0, fmt.Errorf("create article: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read created article ID: %w", err)
	}
	if _, err := syncArticleHighlight(ctx, tx, id, article); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit created article: %w", err)
	}
	return id, nil
}

func UpdateArticle(ctx context.Context, database *sql.DB, article models.Article) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update article: %w", err)
	}
	defer tx.Rollback()
	const query = `
		UPDATE article
		SET title = ?, slug = ?, category = ?, summary = ?, body_md = ?, cover_image = ?, published = ?
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, query, article.Title, article.Slug, article.Category,
		article.Summary, article.BodyMD, article.CoverImage, article.Published, article.ID)
	if err != nil {
		return fmt.Errorf("update article: %w", err)
	}
	if err := requireRowsAffected(result, "article"); err != nil {
		return err
	}
	if _, err := syncArticleHighlight(ctx, tx, article.ID, article); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit updated article: %w", err)
	}
	return nil
}

func DeleteArticle(ctx context.Context, database *sql.DB, id int64) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete article: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM highlight WHERE article_id = ?`, id); err != nil {
		return fmt.Errorf("delete article highlight: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM article WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}
	if err := requireRowsAffected(result, "article"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit deleted article: %w", err)
	}
	return nil
}

func ToggleArticlePublished(ctx context.Context, database *sql.DB, id int64) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin toggle article publication: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE article SET published = 1 - published WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("toggle article publication: %w", err)
	}
	if err := requireRowsAffected(result, "article"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE highlight SET published = (SELECT published FROM article WHERE id = ?) WHERE article_id = ?`, id, id); err != nil {
		return fmt.Errorf("sync article highlight publication: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit article publication: %w", err)
	}
	return nil
}

func syncArticleHighlight(ctx context.Context, tx *sql.Tx, id int64, article models.Article) (bool, error) {
	if !article.FeaturedHighlight {
		if _, err := tx.ExecContext(ctx, `DELETE FROM highlight WHERE article_id = ?`, id); err != nil {
			return false, fmt.Errorf("remove article highlight: %w", err)
		}
		return false, nil
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM highlight WHERE article_id = ?)`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check article highlight: %w", err)
	}
	const query = `
		INSERT INTO highlight (title, kicker, summary, href, image, sort_order, published, article_id)
		VALUES (?, ?, ?, ?, ?, COALESCE((SELECT MAX(sort_order) + 1 FROM highlight), 0), ?, ?)
		ON CONFLICT(article_id) DO UPDATE SET
			title = excluded.title, kicker = excluded.kicker, summary = excluded.summary,
			href = excluded.href, image = excluded.image, published = excluded.published`
	if _, err := tx.ExecContext(ctx, query, article.Title, article.Category, article.Summary,
		"/dispatches/"+article.Slug, article.CoverImage, article.Published, id); err != nil {
		return false, fmt.Errorf("sync article highlight: %w", err)
	}
	return !exists, nil
}

func CountCoverImageReferences(ctx context.Context, database *sql.DB, image string) (int, error) {
	var count int
	const query = `SELECT
		(SELECT COUNT(*) FROM article WHERE cover_image = ?) +
		(SELECT COUNT(*) FROM highlight WHERE image = ?)`
	if err := database.QueryRowContext(ctx, query, image, image).Scan(&count); err != nil {
		return 0, fmt.Errorf("count cover image references: %w", err)
	}
	return count, nil
}

func ListAdminHighlights(ctx context.Context, database *sql.DB) ([]models.Highlight, error) {
	const query = `
		SELECT id, title, kicker, summary, href, image, sort_order, published
		FROM highlight
		ORDER BY sort_order, id`
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query admin highlights: %w", err)
	}
	defer rows.Close()

	var highlights []models.Highlight
	for rows.Next() {
		var highlight models.Highlight
		if err := rows.Scan(&highlight.ID, &highlight.Title, &highlight.Kicker, &highlight.Summary,
			&highlight.Href, &highlight.Image, &highlight.SortOrder, &highlight.Published); err != nil {
			return nil, fmt.Errorf("scan admin highlight: %w", err)
		}
		highlights = append(highlights, highlight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin highlights: %w", err)
	}
	return highlights, nil
}

func GetHighlight(ctx context.Context, database *sql.DB, id int64) (models.Highlight, error) {
	const query = `
		SELECT id, title, kicker, summary, href, image, sort_order, published
		FROM highlight WHERE id = ?`
	var highlight models.Highlight
	err := database.QueryRowContext(ctx, query, id).Scan(&highlight.ID, &highlight.Title,
		&highlight.Kicker, &highlight.Summary, &highlight.Href, &highlight.Image,
		&highlight.SortOrder, &highlight.Published)
	if err != nil {
		return models.Highlight{}, fmt.Errorf("get highlight: %w", err)
	}
	return highlight, nil
}

func CreateHighlight(ctx context.Context, database *sql.DB, highlight models.Highlight) (int64, error) {
	const query = `
		INSERT INTO highlight (title, kicker, summary, href, image, sort_order, published)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := database.ExecContext(ctx, query, highlight.Title, highlight.Kicker, highlight.Summary,
		highlight.Href, highlight.Image, highlight.SortOrder, highlight.Published)
	if err != nil {
		return 0, fmt.Errorf("create highlight: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read created highlight ID: %w", err)
	}
	return id, nil
}

func UpdateHighlight(ctx context.Context, database *sql.DB, highlight models.Highlight) error {
	const query = `
		UPDATE highlight
		SET title = ?, kicker = ?, summary = ?, href = ?, image = ?, sort_order = ?, published = ?
		WHERE id = ?`
	result, err := database.ExecContext(ctx, query, highlight.Title, highlight.Kicker, highlight.Summary,
		highlight.Href, highlight.Image, highlight.SortOrder, highlight.Published, highlight.ID)
	if err != nil {
		return fmt.Errorf("update highlight: %w", err)
	}
	return requireRowsAffected(result, "highlight")
}

func DeleteHighlight(ctx context.Context, database *sql.DB, id int64) error {
	result, err := database.ExecContext(ctx, `DELETE FROM highlight WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete highlight: %w", err)
	}
	return requireRowsAffected(result, "highlight")
}

func ToggleHighlightPublished(ctx context.Context, database *sql.DB, id int64) error {
	result, err := database.ExecContext(ctx, `UPDATE highlight SET published = 1 - published WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("toggle highlight publication: %w", err)
	}
	return requireRowsAffected(result, "highlight")
}

func ListInquiries(ctx context.Context, database *sql.DB) ([]models.Inquiry, error) {
	const query = `
		SELECT id, name, email, subject, message, created_at, is_read
		FROM inquiry ORDER BY created_at DESC, id DESC`
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query inquiries: %w", err)
	}
	defer rows.Close()

	var inquiries []models.Inquiry
	for rows.Next() {
		var inquiry models.Inquiry
		if err := rows.Scan(&inquiry.ID, &inquiry.Name, &inquiry.Email, &inquiry.Subject,
			&inquiry.Message, &inquiry.CreatedAt, &inquiry.Read); err != nil {
			return nil, fmt.Errorf("scan inquiry: %w", err)
		}
		inquiries = append(inquiries, inquiry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inquiries: %w", err)
	}
	return inquiries, nil
}

func CountUnreadInquiries(ctx context.Context, database *sql.DB) (int, error) {
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM inquiry WHERE is_read = 0`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread inquiries: %w", err)
	}
	return count, nil
}

func SetInquiryRead(ctx context.Context, database *sql.DB, id int64, read bool) error {
	result, err := database.ExecContext(ctx, `UPDATE inquiry SET is_read = ? WHERE id = ?`, read, id)
	if err != nil {
		return fmt.Errorf("update inquiry read status: %w", err)
	}
	return requireRowsAffected(result, "inquiry")
}

func ToggleInquiryRead(ctx context.Context, database *sql.DB, id int64) error {
	result, err := database.ExecContext(ctx, `UPDATE inquiry SET is_read = 1 - is_read WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("toggle inquiry read status: %w", err)
	}
	return requireRowsAffected(result, "inquiry")
}

func DeleteInquiry(ctx context.Context, database *sql.DB, id int64) error {
	result, err := database.ExecContext(ctx, `DELETE FROM inquiry WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete inquiry: %w", err)
	}
	return requireRowsAffected(result, "inquiry")
}

func requireRowsAffected(result sql.Result, entity string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected %s count: %w", entity, err)
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
