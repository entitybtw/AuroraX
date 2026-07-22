package combos

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrNotFound indicates a requested combo was not found.
var ErrNotFound = errors.New("combo not found")

type Store interface {
	List(ctx context.Context) ([]Combo, error)
	Get(ctx context.Context, idOrName string) (*Combo, error)
	Upsert(ctx context.Context, combo Combo) error
	Delete(ctx context.Context, idOrName string) error
	Close() error
}

func prepareStoredCombo(combo Combo) Combo {
	combo.ID = strings.TrimSpace(combo.ID)
	combo.Name = strings.TrimSpace(combo.Name)
	combo.Description = strings.TrimSpace(combo.Description)
	combo.Models = normalizeStrings(combo.Models)
	if combo.ID == "" {
		combo.ID = combo.Name
	}
	combo.Source = SourceAdmin
	now := time.Now().UTC()
	if combo.CreatedAt.IsZero() {
		combo.CreatedAt = now
	}
	if combo.UpdatedAt.IsZero() {
		combo.UpdatedAt = now
	}
	return combo
}
