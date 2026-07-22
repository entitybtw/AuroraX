package admin

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/model_aliases"
)

type upsertAliasRequest struct {
	TargetModel    string `json:"target_model"`
	TargetProvider string `json:"target_provider,omitempty"`
	Description    string `json:"description,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
}

func (h *Handler) ListAliases(c *echo.Context) error {
	if h.aliases == nil {
		return handleError(c, featureUnavailableError("aliases feature is unavailable"))
	}
	views := h.aliases.ListViews()
	if views == nil {
		views = []aliases.View{}
	}
	return c.JSON(http.StatusOK, views)
}

// UpsertAlias handles PUT /admin/api/v1/aliases/{name}
func (h *Handler) UpsertAlias(c *echo.Context) error {
	if h.aliases == nil {
		return handleError(c, featureUnavailableError("aliases feature is unavailable"))
	}

	name, err := decodeAliasPathName(c.Param("name"))
	if err != nil {
		return handleError(c, err)
	}

	var req upsertAliasRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	enabled := true
	if existing, ok := h.aliases.Get(name); ok && existing != nil {
		enabled = existing.Enabled
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := h.aliases.Upsert(c.Request().Context(), aliases.Alias{
		Name:           name,
		TargetModel:    req.TargetModel,
		TargetProvider: req.TargetProvider,
		Description:    req.Description,
		Enabled:        enabled,
	}); err != nil {
		return handleError(c, aliasWriteError(err))
	}

	alias, ok := h.aliases.Get(name)
	if !ok {
		return c.NoContent(http.StatusNoContent)
	}
	return c.JSON(http.StatusOK, alias)
}

// DeleteAlias handles DELETE /admin/api/v1/aliases/{name}
func (h *Handler) DeleteAlias(c *echo.Context) error {
	var unavailableErr error
	var deleteFunc func(context.Context, string) error
	if h.aliases == nil {
		unavailableErr = featureUnavailableError("aliases feature is unavailable")
	} else {
		deleteFunc = h.aliases.Delete
	}
	return deleteByName(
		c,
		unavailableErr,
		"name",
		decodeAliasPathName,
		deleteFunc,
		aliases.ErrNotFound,
		"alias not found: ",
		aliasWriteError,
	)
}
