package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/model_overrides"
)

type upsertModelOverrideRequest struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	UserPaths []string `json:"user_paths,omitempty"`
}

func (h *Handler) ListModelOverrides(c *echo.Context) error {
	if h.modelOverrides == nil {
		return handleError(c, featureUnavailableError("model overrides feature is unavailable"))
	}
	views := h.modelOverrides.ListViews()
	if views == nil {
		views = []modeloverrides.View{}
	}
	return c.JSON(http.StatusOK, views)
}

// UpsertModelOverride handles PUT /admin/api/v1/model-overrides/{selector}.
func (h *Handler) UpsertModelOverride(c *echo.Context) error {
	if h.modelOverrides == nil {
		return handleError(c, featureUnavailableError("model overrides feature is unavailable"))
	}

	selector, err := decodeModelOverridePathSelector(c.Param("selector"))
	if err != nil {
		return handleError(c, err)
	}

	var req upsertModelOverrideRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.modelOverrides.Upsert(c.Request().Context(), modeloverrides.Override{
		Selector:  selector,
		Enabled:   &enabled,
		UserPaths: req.UserPaths,
	}); err != nil {
		return handleError(c, modelOverrideWriteError(err))
	}

	override, ok := h.modelOverrides.Get(selector)
	if !ok || override == nil {
		slog.Error("model override service returned no override after upsert", "selector", selector)
		return handleError(c, core.NewProviderError("model_overrides", http.StatusInternalServerError, "model override update failed unexpectedly", nil))
	}
	return c.JSON(http.StatusOK, override)
}

// DeleteModelOverride handles DELETE /admin/api/v1/model-overrides/{selector}.
func (h *Handler) DeleteModelOverride(c *echo.Context) error {
	var unavailableErr error
	var deleteFunc func(context.Context, string) error
	if h.modelOverrides == nil {
		unavailableErr = featureUnavailableError("model overrides feature is unavailable")
	} else {
		deleteFunc = h.modelOverrides.Delete
	}
	return deleteByName(
		c,
		unavailableErr,
		"selector",
		decodeModelOverridePathSelector,
		deleteFunc,
		modeloverrides.ErrNotFound,
		"model override not found: ",
		modelOverrideWriteError,
	)
}
