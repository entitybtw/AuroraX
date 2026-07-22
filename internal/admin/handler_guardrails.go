package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/guardrails"
)

type upsertGuardrailRequest struct {
	Type        string          `json:"type"`
	Direction   string          `json:"direction,omitempty"`
	Description string          `json:"description,omitempty"`
	UserPath    string          `json:"user_path,omitempty"`
	Config      json.RawMessage `json:"config"`
}

func (h *Handler) ListGuardrailTypes(c *echo.Context) error {
	if h.guardrailDefs == nil {
		return handleError(c, featureUnavailableError("guardrails feature is unavailable"))
	}
	return c.JSON(http.StatusOK, h.guardrailDefs.TypeDefinitions())
}

// ListGuardrails handles GET /admin/api/v1/guardrails
func (h *Handler) ListGuardrails(c *echo.Context) error {
	if h.guardrailDefs == nil {
		return handleError(c, featureUnavailableError("guardrails feature is unavailable"))
	}
	views := h.guardrailDefs.ListViews()
	if views == nil {
		views = []guardrails.View{}
	}
	return c.JSON(http.StatusOK, views)
}

// UpsertGuardrail handles PUT /admin/api/v1/guardrails/{name}
func (h *Handler) UpsertGuardrail(c *echo.Context) error {
	if h.guardrailDefs == nil {
		return handleError(c, featureUnavailableError("guardrails feature is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("guardrail name is required", nil))
	}

	var req upsertGuardrailRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	userPath, err := normalizeUserPathQueryParam("user_path", req.UserPath)
	if err != nil {
		return handleError(c, err)
	}

	h.mutationMu.Lock()
	defer h.mutationMu.Unlock()

	if err := h.guardrailDefs.Upsert(c.Request().Context(), guardrails.Definition{
		Name:        name,
		Type:        req.Type,
		Direction:   req.Direction,
		Description: req.Description,
		UserPath:    userPath,
		Config:      req.Config,
	}); err != nil {
		return handleError(c, guardrailWriteError(err))
	}
	if err := h.refreshWorkflowsAfterGuardrailChange(c.Request().Context()); err != nil {
		return handleError(c, err)
	}

	definition, ok := h.guardrailDefs.Get(name)
	if !ok {
		return c.NoContent(http.StatusNoContent)
	}
	return c.JSON(http.StatusOK, guardrails.ViewFromDefinition(*definition))
}

// DeleteGuardrail handles DELETE /admin/api/v1/guardrails/{name}
func (h *Handler) DeleteGuardrail(c *echo.Context) error {
	if h.guardrailDefs == nil {
		return handleError(c, featureUnavailableError("guardrails feature is unavailable"))
	}

	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return handleError(c, core.NewInvalidRequestError("guardrail name is required", nil))
	}

	h.mutationMu.Lock()
	defer h.mutationMu.Unlock()

	referencingWorkflows, err := h.activeWorkflowGuardrailReferences(c.Request().Context(), name)
	if err != nil {
		return handleError(c, err)
	}
	if len(referencingWorkflows) > 0 {
		return handleError(c, core.NewInvalidRequestError("guardrail is used by active workflows: "+strings.Join(referencingWorkflows, ", "), nil))
	}

	if err := h.guardrailDefs.Delete(c.Request().Context(), name); err != nil {
		if errors.Is(err, guardrails.ErrNotFound) {
			return handleError(c, core.NewNotFoundError("guardrail not found: "+name))
		}
		return handleError(c, guardrailWriteError(err))
	}
	if err := h.refreshWorkflowsAfterGuardrailChange(c.Request().Context()); err != nil {
		return handleError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}
