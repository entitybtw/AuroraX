package combos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLStore stores admin-created combos in PostgreSQL.
type PostgreSQLStore struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLStore creates the combos table and indexes if needed.
func NewPostgreSQLStore(ctx context.Context, pool *pgxpool.Pool) (*PostgreSQLStore, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}

	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS combos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			models_json JSONB NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			source TEXT NOT NULL DEFAULT 'admin',
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create combos table: %w", err)
	}
	if _, err := pool.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_combos_name ON combos(name)`); err != nil {
		return nil, fmt.Errorf("failed to create combos name index: %w", err)
	}
	if _, err := pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_combos_enabled ON combos(enabled)`); err != nil {
		return nil, fmt.Errorf("failed to create combos enabled index: %w", err)
	}
	if _, err := pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_combos_updated_at ON combos(updated_at DESC)`); err != nil {
		return nil, fmt.Errorf("failed to create combos updated_at index: %w", err)
	}
	return &PostgreSQLStore{pool: pool}, nil
}

func (s *PostgreSQLStore) List(ctx context.Context) ([]Combo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, models_json::text, enabled, source, created_at, updated_at
		FROM combos
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer rows.Close()

	out := make([]Combo, 0)
	for rows.Next() {
		combo, err := scanPostgreSQLCombo(rows)
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

func (s *PostgreSQLStore) Get(ctx context.Context, idOrName string) (*Combo, error) {
	key := strings.TrimSpace(idOrName)
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, description, models_json::text, enabled, source, created_at, updated_at
		FROM combos
		WHERE id = $1 OR name = $1
		LIMIT 1
	`, key)
	combo, err := scanPostgreSQLCombo(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &combo, nil
}

func (s *PostgreSQLStore) Upsert(ctx context.Context, combo Combo) error {
	combo = prepareStoredCombo(combo)
	modelsJSON, err := json.Marshal(combo.Models)
	if err != nil {
		return fmt.Errorf("marshal combo models: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO combos (id, name, description, models_json, enabled, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			models_json = excluded.models_json,
			enabled = excluded.enabled,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, combo.ID, combo.Name, combo.Description, string(modelsJSON), combo.Enabled, combo.Source, combo.CreatedAt.Unix(), combo.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert combo: %w", err)
	}
	return nil
}

func (s *PostgreSQLStore) Delete(ctx context.Context, idOrName string) error {
	key := strings.TrimSpace(idOrName)
	cmd, err := s.pool.Exec(ctx, `DELETE FROM combos WHERE id = $1 OR name = $1`, key)
	if err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgreSQLStore) Close() error { return nil }

func scanPostgreSQLCombo(scanner interface{ Scan(dest ...any) error }) (Combo, error) {
	var combo Combo
	var modelsJSON string
	var createdAt int64
	var updatedAt int64
	if err := scanner.Scan(
		&combo.ID,
		&combo.Name,
		&combo.Description,
		&modelsJSON,
		&combo.Enabled,
		&combo.Source,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Combo{}, err
	}
	if err := json.Unmarshal([]byte(modelsJSON), &combo.Models); err != nil {
		return Combo{}, fmt.Errorf("unmarshal combo models: %w", err)
	}
	combo.CreatedAt = time.Unix(createdAt, 0).UTC()
	combo.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return combo, nil
}
