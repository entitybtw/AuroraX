package combos

import (
	"time"

	"aurora/internal/core"
)

const (
	SourceStatic = "static"
	SourceAdmin  = "admin"
)

type Combo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Models      []string  `json:"models"`
	Enabled     bool      `json:"enabled"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type View struct {
	Combo     Combo    `json:"combo"`
	Valid     bool     `json:"valid"`
	Errors    []string `json:"errors,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Primary   string   `json:"primary,omitempty"`
	Fallbacks []string `json:"fallbacks,omitempty"`
	Readonly  bool     `json:"readonly"`
}

type Catalog interface {
	Supports(model string) bool
	LookupModel(model string) (*core.Model, bool)
}

type DownstreamResolver interface {
	ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error)
}
