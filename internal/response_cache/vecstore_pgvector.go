package responsecache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"aurora/configuration"
)

type pgVecStore struct {
	pool      *pgxpool.Pool
	table     string
	dim       int
	cleanup   *vecJanitor
	quotedTbl string
}

func newPGVectorStore(cfg config.PGVectorConfig) (*pgVecStore, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("vecstore pgvector: url is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("vecstore pgvector: dimension must be > 0")
	}
	tbl := strings.TrimSpace(cfg.Table)
	if tbl == "" {
		tbl = "aurora_semantic_cache"
	}
	if err := validatePGIdentifier(tbl); err != nil {
		return nil, fmt.Errorf("vecstore pgvector: table: %w", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("vecstore pgvector: connect: %w", err)
	}
	quoted := quotePGIdent(tbl)
	s := &pgVecStore{
		pool:      pool,
		table:     tbl,
		dim:       cfg.Dimension,
		quotedTbl: quoted,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("vecstore pgvector: create extension: %w", err)
	}
	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	cache_key   TEXT NOT NULL,
	params_hash TEXT NOT NULL,
	embedding   vector(%d) NOT NULL,
	response    BYTEA NOT NULL,
	prompt_text TEXT NOT NULL DEFAULT '',
	expires_at  BIGINT NOT NULL,
	PRIMARY KEY (cache_key, params_hash)
)`, quoted, cfg.Dimension)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		pool.Close()
		return nil, fmt.Errorf("vecstore pgvector: create table: %w", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS prompt_text TEXT NOT NULL DEFAULT ''`, quoted)); err != nil {
		pool.Close()
		return nil, fmt.Errorf("vecstore pgvector: add prompt_text column: %w", err)
	}
	idxStmts := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (params_hash)`, quotePGIdent(tbl+"_params_hash_idx"), quoted),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (expires_at)`, quotePGIdent(tbl+"_expires_at_idx"), quoted),
	}
	for _, stmt := range idxStmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			pool.Close()
			return nil, fmt.Errorf("vecstore pgvector: create index: %w", err)
		}
	}
	s.cleanup = startVecCleanup(s)
	return s, nil
}

func validatePGIdentifier(name string) error {
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("invalid identifier %q (use letters, digits, underscore only)", name)
	}
	if name == "" {
		return fmt.Errorf("empty identifier")
	}
	return nil
}

func quotePGIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (s *pgVecStore) Close() error {
	s.cleanup.close()
	s.pool.Close()
	return nil
}

func (s *pgVecStore) Insert(ctx context.Context, key string, vec []float32, response []byte, paramsHash string, promptText string, ttl time.Duration) error {
	if len(vec) != s.dim {
		return fmt.Errorf("vecstore pgvector: embedding len %d != configured dimension %d", len(vec), s.dim)
	}
	var expiresAt int64
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}
	vecLit := pgvectorLiteral(vec)
	q := fmt.Sprintf(`
INSERT INTO %s (cache_key, params_hash, embedding, response, prompt_text, expires_at)
VALUES ($1, $2, $3::vector, $4, $5, $6)
ON CONFLICT (cache_key, params_hash) DO UPDATE SET
	embedding = EXCLUDED.embedding,
	response = EXCLUDED.response,
	prompt_text = EXCLUDED.prompt_text,
		expires_at = EXCLUDED.expires_at`, s.quotedTbl)
	_, err := s.pool.Exec(ctx, q, key, paramsHash, vecLit, response, promptText, expiresAt)
	if err != nil {
		return fmt.Errorf("vecstore pgvector: insert: %w", err)
	}
	return nil
}

func (s *pgVecStore) Search(ctx context.Context, vec []float32, paramsHash string, limit int) ([]VecResult, error) {
	if len(vec) != s.dim {
		return nil, fmt.Errorf("vecstore pgvector: embedding len %d != dimension %d", len(vec), s.dim)
	}
	now := time.Now().Unix()
	vecLit := pgvectorLiteral(vec)
	q := fmt.Sprintf(`
SELECT cache_key, response, prompt_text,
	GREATEST(0::double precision, LEAST(1::double precision, 1 - (embedding <=> $1::vector))) AS score
FROM %s
WHERE params_hash = $2 AND (expires_at = 0 OR expires_at >= $3)
ORDER BY embedding <=> $1::vector
LIMIT $4`, s.quotedTbl)
	rows, err := s.pool.Query(ctx, q, vecLit, paramsHash, now, limit)
	if err != nil {
		return nil, fmt.Errorf("vecstore pgvector: search: %w", err)
	}
	defer rows.Close()
	var out []VecResult
	for rows.Next() {
		var k string
		var resp []byte
		var promptText string
		var score float64
		if err := rows.Scan(&k, &resp, &promptText, &score); err != nil {
			return nil, err
		}
		out = append(out, VecResult{Key: k, Score: float32(score), Response: resp, PromptText: promptText})
	}
	return out, rows.Err()
}

func (s *pgVecStore) DeleteExpired(ctx context.Context) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE expires_at > 0 AND expires_at < $1`, s.quotedTbl)
	_, err := s.pool.Exec(ctx, q, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("vecstore pgvector: delete expired: %w", err)
	}
	return nil
}
