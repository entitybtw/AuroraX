package combos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"aurora/configuration"
	"aurora/internal/storage"
)

// Result holds the initialized combos service and any owned resources.
type Result struct {
	Service *Service
	Store   Store
	Storage storage.Storage

	closeOnce sync.Once
	closeErr  error
}

// Close releases resources held by the combos subsystem.
func (r *Result) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		var errs []error
		if r.Storage != nil {
			if err := r.Storage.Close(); err != nil {
				errs = append(errs, fmt.Errorf("storage close: %w", err))
			}
		} else if r.Store != nil {
			if err := r.Store.Close(); err != nil {
				errs = append(errs, fmt.Errorf("store close: %w", err))
			}
		}
		if len(errs) > 0 {
			r.closeErr = fmt.Errorf("close errors: %w", errors.Join(errs...))
		}
	})
	return r.closeErr
}

// New creates a combos subsystem with its own storage connection.
func New(ctx context.Context, cfg *config.Config, catalog Catalog, downstream DownstreamResolver, static []Combo) (*Result, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	storeConn, err := storage.New(ctx, cfg.Storage.BackendConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	result, err := newResult(ctx, cfg, storeConn, catalog, downstream, static)
	if err != nil {
		_ = storeConn.Close()
		return nil, err
	}
	result.Storage = storeConn
	return result, nil
}

// NewWithSharedStorage creates a combos subsystem using an existing storage connection.
func NewWithSharedStorage(ctx context.Context, cfg *config.Config, shared storage.Storage, catalog Catalog, downstream DownstreamResolver, static []Combo) (*Result, error) {
	if shared == nil {
		return nil, fmt.Errorf("shared storage is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	return newResult(ctx, cfg, shared, catalog, downstream, static)
}

func newResult(ctx context.Context, cfg *config.Config, storeConn storage.Storage, catalog Catalog, downstream DownstreamResolver, static []Combo) (*Result, error) {
	_ = cfg
	store, err := createStore(ctx, storeConn)
	if err != nil {
		return nil, err
	}
	service, err := NewServiceWithStatic(store, catalog, downstream, static)
	if err != nil {
		return nil, err
	}
	if err := service.Refresh(ctx); err != nil {
		return nil, err
	}

	return &Result{
		Service: service,
		Store:   store,
	}, nil
}

func createStore(ctx context.Context, store storage.Storage) (Store, error) {
	return storage.ResolveBackend[Store](
		store,
		func(db *sql.DB) (Store, error) { return NewSQLiteStore(db) },
		func(pool *pgxpool.Pool) (Store, error) { return NewPostgreSQLStore(ctx, pool) },
		func(db *mongo.Database) (Store, error) { return NewMongoDBStore(db) },
	)
}
