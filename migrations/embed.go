package migrations

import "embed"

// Files embeds SQL migrations into the application binary.
//
//go:embed *.sql
var Files embed.FS
