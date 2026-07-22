package scope

import (
	"context"
	"time"

	"aurora/configuration"
	"aurora/internal/storage"
)

const (
	DefaultID   = "default"
	DefaultSlug = "default"
)

type contextKey struct{}

func WithTenantID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = DefaultID
	}
	return context.WithValue(ctx, contextKey{}, id)
}

func EffectiveID(ctx context.Context) string {
	if id := TenantIDFromContext(ctx); id != "" {
		return id
	}
	return DefaultID
}

func TenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(contextKey{}).(string); ok && id != "" {
		return id
	}
	return ""
}

type Status string

const StatusActive Status = "active"

type Metadata struct {
	ExternalID string `json:"external_id,omitempty" bson:"external_id,omitempty"`
	Plan       string `json:"plan,omitempty" bson:"plan,omitempty"`
}

type Tenant struct {
	ID        string    `json:"id" bson:"_id"`
	Slug      string    `json:"slug" bson:"slug"`
	Name      string    `json:"name" bson:"name"`
	Status    Status    `json:"status" bson:"status"`
	Metadata  Metadata  `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type Service struct{}

func (s *Service) Get(context.Context, string) (Tenant, error) {
	return Tenant{ID: DefaultID, Slug: DefaultSlug, Name: "Default", Status: StatusActive}, nil
}

type Result struct {
	Service *Service
	Storage storage.Storage
}

func New(context.Context, *config.Config) (*Result, error) { return &Result{Service: &Service{}}, nil }
func NewWithSharedStorage(context.Context, storage.Storage) (*Result, error) {
	return &Result{Service: &Service{}}, nil
}
func (r *Result) Close() error { return nil }
