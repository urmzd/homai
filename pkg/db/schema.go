package db

import (
	"context"
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 1

// Schema SQL for version 1
const schemaV1 = `
-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Profiles (multi-installation support)
CREATE TABLE IF NOT EXISTS profiles (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    is_active   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- API server config
CREATE TABLE IF NOT EXISTS api_servers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id  INTEGER NOT NULL UNIQUE REFERENCES profiles(id) ON DELETE CASCADE,
    host        TEXT NOT NULL DEFAULT '0.0.0.0',
    port        INTEGER NOT NULL DEFAULT 8080,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Devices
CREATE TABLE IF NOT EXISTS devices (
    id           TEXT PRIMARY KEY,
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT '',
    protocol     TEXT NOT NULL DEFAULT '',
    manufacturer TEXT NOT NULL DEFAULT '',
    model        TEXT NOT NULL DEFAULT '',
    exposes      TEXT NOT NULL DEFAULT '[]',
    state_schema TEXT NOT NULL DEFAULT '{}',
    state        TEXT NOT NULL DEFAULT '{}',
    last_seen    TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_profiles_active ON profiles(is_active);
CREATE INDEX IF NOT EXISTS idx_devices_profile ON devices(profile_id);
CREATE INDEX IF NOT EXISTS idx_devices_name ON devices(name);
`

// Migrate runs database migrations to bring the schema up to date.
func (db *DB) Migrate(ctx context.Context) error {
	version, err := db.getSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	if version >= currentSchemaVersion {
		return nil // Already up to date
	}

	if version < 1 {
		if err := db.applySchemaV1(ctx); err != nil {
			return fmt.Errorf("failed to apply schema v1: %w", err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version, or 0 if no schema exists.
func (db *DB) getSchemaVersion(ctx context.Context) (int, error) {
	// Check if schema_version table exists
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='schema_version'
	`).Scan(&count)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	var version int
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

// applySchemaV1 applies the initial schema.
func (db *DB) applySchemaV1(ctx context.Context) error {
	return db.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, schemaV1); err != nil {
			return fmt.Errorf("failed to execute schema: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (1)`); err != nil {
			return fmt.Errorf("failed to record schema version: %w", err)
		}

		return nil
	})
}

// SchemaVersion returns the current schema version.
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	return db.getSchemaVersion(ctx)
}
