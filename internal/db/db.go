package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CodeToole/waitaminutedigital_go/migrations"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func Migrate(ctx context.Context, database *sql.DB) error {
	schema, err := migrations.Files.ReadFile("001_init.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}
	if _, err := database.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply database schema: %w", err)
	}
	rows, err := database.QueryContext(ctx, `PRAGMA table_info(highlight)`)
	if err != nil {
		return fmt.Errorf("inspect highlight schema: %w", err)
	}
	articleIDColumn := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("read highlight schema: %w", err)
		}
		if name == "article_id" {
			articleIDColumn = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate highlight schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close highlight schema: %w", err)
	}
	if !articleIDColumn {
		migration, err := migrations.Files.ReadFile("002_highlight_article.sql")
		if err != nil {
			return fmt.Errorf("read highlight migration: %w", err)
		}
		if _, err := database.ExecContext(ctx, string(migration)); err != nil {
			return fmt.Errorf("apply highlight migration: %w", err)
		}
	}
	return nil
}
