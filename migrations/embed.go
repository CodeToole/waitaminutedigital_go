package migrations

import "embed"

// Files embeds SQL migrations into the application binary.
//
//go:embed 001_init.sql 002_highlight_article.sql
var Files embed.FS
