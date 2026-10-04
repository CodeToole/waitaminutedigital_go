package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	// A single connection keeps SQLite access serialized and ensures in-memory databases
	// remain consistent. WAL is intentionally disabled because the live DB is on Azure
	// App Service network storage, where WAL's shared-memory files are not reliable.
	database.SetMaxOpenConns(1)

	if _, err := database.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		database.Close()
		return nil, fmt.Errorf("set database busy timeout: %w", err)
	}
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
	if _, err := database.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	trackingTableExists, err := tableExists(ctx, database, "schema_migrations")
	if err != nil {
		return fmt.Errorf("check migration tracking table: %w", err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}

	if !trackingTableExists {
		if err := baselineLegacySchema(ctx, database); err != nil {
			return err
		}
	}

	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	type migrationFile struct {
		version int
		name    string
	}
	files := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		versionText, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			continue
		}
		version, err := strconv.Atoi(versionText)
		if err != nil || version <= 0 {
			return fmt.Errorf("invalid migration filename %q: expected a positive numeric prefix", entry.Name())
		}
		files = append(files, migrationFile{version: version, name: entry.Name()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].version < files[j].version
	})
	for i := 1; i < len(files); i++ {
		if files[i-1].version == files[i].version {
			return fmt.Errorf("duplicate migration version %d", files[i].version)
		}
	}

	for _, file := range files {
		if err := applyMigration(ctx, database, file.version, file.name); err != nil {
			return err
		}
	}
	return nil
}

func baselineLegacySchema(ctx context.Context, database *sql.DB) error {
	articleTableExists, err := tableExists(ctx, database, "article")
	if err != nil {
		return fmt.Errorf("check legacy article table: %w", err)
	}
	if !articleTableExists {
		return nil
	}

	versions := []int{1}
	articleIDColumn, err := columnExists(ctx, database, "highlight", "article_id")
	if err != nil {
		return fmt.Errorf("check legacy highlight schema: %w", err)
	}
	if articleIDColumn {
		versions = append(versions, 2)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legacy migration baseline: %w", err)
	}
	for _, version := range versions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schema_migrations (version, applied_at)
			VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record legacy migration %d: %w", version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy migration baseline: %w", err)
	}
	return nil
}

func applyMigration(ctx context.Context, database *sql.DB, version int, name string) error {
	sqlFile, err := migrations.Files.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	var applied bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM schema_migrations WHERE version = ?
	)`, version).Scan(&applied); err != nil {
		tx.Rollback()
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("finish migration check %s: %w", name, err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, string(sqlFile)); err != nil {
		tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, applied_at)
		VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, version); err != nil {
		tx.Rollback()
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}

func tableExists(ctx context.Context, database *sql.DB, name string) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sqlite_master
			WHERE type = 'table' AND name = ?
		)`, name).Scan(&exists)
	return exists, err
}

func columnExists(ctx context.Context, database *sql.DB, table, column string) (bool, error) {
	rows, err := database.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
