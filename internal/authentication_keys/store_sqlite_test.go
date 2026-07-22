package authkeys

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteStorePersistsTenantIDAndRateLimits(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	createdAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	key := AuthKey{
		ID:            "key-1",
		Name:          "primary",
		RedactedValue: TokenPrefix + "...abcd",
		SecretHash:    hashSecret("secret"),
		Enabled:       true,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
		TenantID:      "tenant-a",
		RateLimits: RateLimits{
			RequestsPerMinute: 12,
			RequestsPerDay:    1000,
			TokensPerMinute:   40000,
			TokensPerDay:      500000,
		},
	}
	if err := store.Create(context.Background(), key); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	keys, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List() len = %d, want 1", len(keys))
	}
	if keys[0].TenantID != "tenant-a" {
		t.Fatalf("TenantID = %q, want tenant-a", keys[0].TenantID)
	}
	if keys[0].RateLimits != key.RateLimits {
		t.Fatalf("RateLimits = %#v, want %#v", keys[0].RateLimits, key.RateLimits)
	}
}

func TestNewSQLiteStoreAddsMissingRateLimitColumns(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		CREATE TABLE auth_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			user_path TEXT,
			redacted_value TEXT NOT NULL,
			secret_hash TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			expires_at INTEGER,
			deactivated_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create legacy auth_keys table: %v", err)
	}

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewSQLiteStore() = nil, want store")
	}

	for _, column := range []string{"requests_per_minute", "requests_per_day", "tokens_per_minute", "tokens_per_day", "tenant_id"} {
		if !sqliteColumnExists(t, db, "auth_keys", column) {
			t.Fatalf("column %q missing after initialization", column)
		}
	}
}

func sqliteColumnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info('` + table + `')`)
	if err != nil {
		t.Fatalf("PRAGMA table_info() error = %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	return false
}
