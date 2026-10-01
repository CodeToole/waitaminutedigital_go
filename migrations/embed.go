package migrations

import "embed"

// Files embeds SQL migrations into the application binary.
//
//go:embed 001_init.sql
var Files embed.FS
