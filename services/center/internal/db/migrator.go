package db

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/migrations"
)

type MigrationState struct {
	Status         string `json:"status"`
	CurrentVersion int    `json:"currentVersion"`
	LatestVersion  int    `json:"latestVersion"`
}

type Migrator struct{}

func NewMigrator() Migrator {
	return Migrator{}
}

func (m Migrator) Apply(ctx context.Context, pool *pgxpool.Pool) (MigrationState, error) {
	files, err := loadMigrationFiles()
	if err != nil {
		return MigrationState{}, err
	}

	if err := ensureVersionTable(ctx, pool); err != nil {
		return MigrationState{}, err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return MigrationState{}, err
	}

	for _, file := range files {
		if applied[file.Version] {
			continue
		}

		if _, err := pool.Exec(ctx, string(file.Content)); err != nil {
			return MigrationState{}, fmt.Errorf("apply migration %s: %w", file.Name, err)
		}

		if _, err := pool.Exec(
			ctx,
			"INSERT INTO schema_migrations(version, name) VALUES ($1, $2)",
			file.Version,
			file.Name,
		); err != nil {
			return MigrationState{}, fmt.Errorf("record migration %s: %w", file.Name, err)
		}
	}

	return m.State(ctx, pool)
}

func (m Migrator) State(ctx context.Context, pool *pgxpool.Pool) (MigrationState, error) {
	files, err := loadMigrationFiles()
	if err != nil {
		return MigrationState{}, err
	}

	if err := ensureVersionTable(ctx, pool); err != nil {
		return MigrationState{}, err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return MigrationState{}, err
	}

	currentVersion := 0
	for version := range applied {
		if version > currentVersion {
			currentVersion = version
		}
	}

	latestVersion := 0
	for _, file := range files {
		if file.Version > latestVersion {
			latestVersion = file.Version
		}
	}

	status := "ready"
	if currentVersion < latestVersion {
		status = "pending"
	}

	return MigrationState{
		Status:         status,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
	}, nil
}

type migrationFile struct {
	Name    string
	Version int
	Content []byte
}

func loadMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	files := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}

		content, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		files = append(files, migrationFile{
			Name:    entry.Name(),
			Version: version,
			Content: content,
		})
	}

	slices.SortFunc(files, func(a migrationFile, b migrationFile) int {
		return a.Version - b.Version
	})

	return files, nil
}

func parseMigrationVersion(name string) (int, error) {
	prefix, _, _ := strings.Cut(name, "_")
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("parse migration version for %s: %w", name, err)
	}
	return version, nil
}

func ensureVersionTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}
