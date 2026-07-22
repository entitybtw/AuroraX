package combos

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SQLiteStore stores admin-created combos in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates the combos table and indexes if needed.
func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS combos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			models_json TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			source TEXT NOT NULL DEFAULT 'admin',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create combos table: %w", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_combos_name ON combos(name)`); err != nil {
		return nil, fmt.Errorf("failed to create combos name index: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_combos_enabled ON combos(enabled)`); err != nil {
		return nil, fmt.Errorf("failed to create combos enabled index: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_combos_updated_at ON combos(updated_at DESC)`); err != nil {
		return nil, fmt.Errorf("failed to create combos updated_at index: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]Combo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, models_json, enabled, source, created_at, updated_at
		FROM combos
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer rows.Close()

	out := make([]Combo, 0)
	for rows.Next() {
		combo, err := scanSQLiteCombo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan combo: %w", err)
		}
		out = append(out, combo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate combos: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) Get(ctx context.Context, idOrName string) (*Combo, error) {
	key := strings.TrimSpace(idOrName)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, models_json, enabled, source, created_at, updated_at
		FROM combos
		WHERE id = ? OR name = ?
		LIMIT 1
	`, key, key)
	combo, err := scanSQLiteCombo(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &combo, nil
}

func (s *SQLiteStore) Upsert(ctx context.Context, combo Combo) error {
	combo = prepareStoredCombo(combo)
	modelsJSON, err := json.Marshal(combo.Models)
	if err != nil {
		return fmt.Errorf("marshal combo models: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO combos (id, name, description, models_json, enabled, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			models_json = excluded.models_json,
			enabled = excluded.enabled,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, combo.ID, combo.Name, combo.Description, string(modelsJSON), boolToSQLite(combo.Enabled), combo.Source, combo.CreatedAt.Unix(), combo.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert combo: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Delete(ctx context.Context, idOrName string) error {
	key := strings.TrimSpace(idOrName)
	result, err := s.db.ExecContext(ctx, `DELETE FROM combos WHERE id = ? OR name = ?`, key, key)
	if err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) Close() error { return nil }

func scanSQLiteCombo(scanner interface{ Scan(dest ...any) error }) (Combo, error) {
	var combo Combo
	var modelsJSON string
	var enabled int
	var createdAt int64
	var updatedAt int64
	if err := scanner.Scan(
		&combo.ID,
		&combo.Name,
		&combo.Description,
		&modelsJSON,
		&enabled,
		&combo.Source,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Combo{}, err
	}
	if err := json.Unmarshal([]byte(modelsJSON), &combo.Models); err != nil {
		return Combo{}, fmt.Errorf("unmarshal combo models: %w", err)
	}
	combo.Enabled = enabled != 0
	combo.CreatedAt = time.Unix(createdAt, 0).UTC()
	combo.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return combo, nil
}

func boolToSQLite(v bool) int {
	if v {
		return 1
	}
	return 0
}
