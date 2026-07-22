package combos

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"aurora/internal/core"

	_ "modernc.org/sqlite"
)

func TestSQLiteStore_UpsertPersistsComboAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "combos.db")
	ctx := context.Background()

	store := openSQLiteComboStore(t, path)
	createdAt := time.Unix(100, 0).UTC()
	updatedAt := time.Unix(200, 0).UTC()
	combo := Combo{
		ID:          "persisted-id",
		Name:        "persisted-name",
		Description: "survives reopen",
		Models:      []string{"openai/gpt-4o", "anthropic/claude-sonnet"},
		Enabled:     true,
		Source:      SourceAdmin,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	if err := store.Upsert(ctx, combo); err != nil {
		t.Fatalf("upsert combo: %v", err)
	}
	closeSQLiteStore(t, store)

	reopened := openSQLiteComboStore(t, path)
	defer closeSQLiteStore(t, reopened)

	byID, err := reopened.Get(ctx, combo.ID)
	if err != nil {
		t.Fatalf("get combo by id after reopen: %v", err)
	}
	assertComboMatches(t, byID, combo)

	byName, err := reopened.Get(ctx, combo.Name)
	if err != nil {
		t.Fatalf("get combo by name after reopen: %v", err)
	}
	assertComboMatches(t, byName, combo)
}

func TestSQLiteStore_UpsertReplacesModels(t *testing.T) {
	store := openSQLiteComboStore(t, filepath.Join(t.TempDir(), "combos.db"))
	defer closeSQLiteStore(t, store)
	ctx := context.Background()

	initial := Combo{ID: "replace", Name: "replace", Models: []string{"a/model", "b/model"}, Enabled: true}
	if err := store.Upsert(ctx, initial); err != nil {
		t.Fatalf("upsert initial combo: %v", err)
	}
	replacement := Combo{ID: "replace", Name: "replace", Description: "new", Models: []string{"c/model", "d/model"}, Enabled: false}
	if err := store.Upsert(ctx, replacement); err != nil {
		t.Fatalf("upsert replacement combo: %v", err)
	}

	actual, err := store.Get(ctx, "replace")
	if err != nil {
		t.Fatalf("get replacement combo: %v", err)
	}
	if actual.Description != "new" {
		t.Fatalf("description = %q, want new", actual.Description)
	}
	if !reflect.DeepEqual(actual.Models, replacement.Models) {
		t.Fatalf("models = %#v, want %#v", actual.Models, replacement.Models)
	}
	if actual.Enabled {
		t.Fatalf("enabled = true, want false")
	}
}

func TestSQLiteStore_DeletePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "combos.db")
	ctx := context.Background()
	store := openSQLiteComboStore(t, path)
	if err := store.Upsert(ctx, Combo{ID: "delete-id", Name: "delete-name", Models: []string{"a/model", "b/model"}, Enabled: true}); err != nil {
		t.Fatalf("upsert combo: %v", err)
	}
	if err := store.Delete(ctx, "delete-name"); err != nil {
		t.Fatalf("delete combo: %v", err)
	}
	closeSQLiteStore(t, store)

	reopened := openSQLiteComboStore(t, path)
	defer closeSQLiteStore(t, reopened)
	_, err := reopened.Get(ctx, "delete-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted combo error = %v, want ErrNotFound", err)
	}
}

func TestSQLiteStore_ListReturnsNameOrder(t *testing.T) {
	store := openSQLiteComboStore(t, filepath.Join(t.TempDir(), "combos.db"))
	defer closeSQLiteStore(t, store)
	ctx := context.Background()

	for _, combo := range []Combo{
		{ID: "z", Name: "zeta", Models: []string{"a/model", "b/model"}, Enabled: true},
		{ID: "a", Name: "alpha", Models: []string{"a/model", "b/model"}, Enabled: true},
	} {
		if err := store.Upsert(ctx, combo); err != nil {
			t.Fatalf("upsert %s: %v", combo.Name, err)
		}
	}
	combos, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list combos: %v", err)
	}
	if got := []string{combos[0].Name, combos[1].Name}; !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("list order = %#v, want alpha,zeta", got)
	}
}

func TestServiceWithStatic_KeepsStaticReadonlyAndMergesPersistedCombos(t *testing.T) {
	store := openSQLiteComboStore(t, filepath.Join(t.TempDir(), "combos.db"))
	defer closeSQLiteStore(t, store)
	catalog := comboTestCatalog{supported: map[string]struct{}{"a/model": {}, "b/model": {}, "c/model": {}, "d/model": {}}}
	static := []Combo{{ID: "static", Name: "static", Models: []string{"a/model", "b/model"}, Enabled: true, Source: SourceStatic}}
	service, err := NewServiceWithStatic(store, catalog, nil, static)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	if err := service.Upsert(context.Background(), Combo{Name: "admin", Models: []string{"c/model", "d/model"}, Enabled: true}); err != nil {
		t.Fatalf("upsert admin combo: %v", err)
	}
	if err := service.Upsert(context.Background(), Combo{Name: "static", Models: []string{"c/model", "d/model"}, Enabled: true}); err == nil {
		t.Fatalf("upsert static combo succeeded, want readonly error")
	}

	views := service.ListViews()
	if len(views) != 2 {
		t.Fatalf("views len = %d, want 2", len(views))
	}
	readonlyByName := map[string]bool{}
	for _, view := range views {
		readonlyByName[view.Combo.Name] = view.Readonly
	}
	if !readonlyByName["static"] {
		t.Fatalf("static combo is not readonly")
	}
	if readonlyByName["admin"] {
		t.Fatalf("admin combo is readonly")
	}
}

func openSQLiteComboStore(t *testing.T, path string) *SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	store, err := NewSQLiteStore(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("new sqlite store: %v", err)
	}
	return store
}

func closeSQLiteStore(t *testing.T, store *SQLiteStore) {
	t.Helper()
	if err := store.db.Close(); err != nil {
		t.Fatalf("close sqlite db: %v", err)
	}
}

func assertComboMatches(t *testing.T, actual *Combo, expected Combo) {
	t.Helper()
	if actual == nil {
		t.Fatalf("actual combo is nil")
	}
	if actual.ID != expected.ID || actual.Name != expected.Name || actual.Description != expected.Description || actual.Enabled != expected.Enabled || actual.Source != expected.Source {
		t.Fatalf("combo = %#v, want %#v", *actual, expected)
	}
	if !reflect.DeepEqual(actual.Models, expected.Models) {
		t.Fatalf("models = %#v, want %#v", actual.Models, expected.Models)
	}
	if !actual.CreatedAt.Equal(expected.CreatedAt) || !actual.UpdatedAt.Equal(expected.UpdatedAt) {
		t.Fatalf("times = (%s,%s), want (%s,%s)", actual.CreatedAt, actual.UpdatedAt, expected.CreatedAt, expected.UpdatedAt)
	}
}

type comboTestCatalog struct {
	supported map[string]struct{}
}

func (c comboTestCatalog) Supports(model string) bool {
	_, ok := c.supported[model]
	return ok
}

func (c comboTestCatalog) LookupModel(model string) (*core.Model, bool) {
	if !c.Supports(model) {
		return nil, false
	}
	return &core.Model{ID: model}, true
}
