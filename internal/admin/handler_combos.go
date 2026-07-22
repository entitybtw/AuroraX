package admin

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/model_combinations"
)

type comboPayload struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Models      []string `json:"models"`
	Enabled     *bool    `json:"enabled,omitempty"`
}

func (h *Handler) ListCombos(c *echo.Context) error {
	if h.combos == nil {
		return c.JSON(http.StatusOK, map[string]any{"combos": []any{}})
	}
	return c.JSON(http.StatusOK, map[string]any{"combos": h.combos.ListViews()})
}

func (h *Handler) GetCombo(c *echo.Context) error {
	if h.combos == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combos are not enabled", nil))
	}
	combo, ok := h.combos.Get(c.Param("id"))
	if !ok {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combo not found", nil))
	}
	return c.JSON(http.StatusOK, h.combos.Validate(*combo))
}

func (h *Handler) CreateCombo(c *echo.Context) error {
	return h.upsertComboWithID(c, "", "")
}

func (h *Handler) UpdateCombo(c *echo.Context) error {
	if h.combos == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combos are not enabled", nil))
	}
	target := strings.TrimSpace(c.Param("id"))
	if existing, ok := h.combos.Get(target); ok {
		if existing.Source == combos.SourceStatic {
			return handleError(c, core.NewInvalidRequestError("static combo cannot be modified through admin API", nil))
		}
		return h.upsertComboWithID(c, target, existing.ID)
	}
	return h.upsertComboWithID(c, target, target)
}

func (h *Handler) ValidateCombo(c *echo.Context) error {
	if h.combos == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combos are not enabled", nil))
	}
	combo, err := comboFromRequest(c, c.Param("id"))
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, h.combos.Validate(combo))
}

func (h *Handler) DeleteCombo(c *echo.Context) error {
	if h.combos == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combos are not enabled", nil))
	}
	if err := h.combos.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), err))
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) upsertComboWithID(c *echo.Context, fallbackName, id string) error {
	if h.combos == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "combos are not enabled", nil))
	}
	combo, err := comboFromRequest(c, fallbackName)
	if err != nil {
		return handleError(c, err)
	}
	if id = strings.TrimSpace(id); id != "" {
		combo.ID = id
	}
	if err := h.combos.Upsert(c.Request().Context(), combo); err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), err))
	}
	created, _ := h.combos.Get(combo.Name)
	return c.JSON(http.StatusOK, h.combos.Validate(*created))
}

func comboFromRequest(c *echo.Context, fallbackName string) (combos.Combo, error) {
	var payload comboPayload
	if err := c.Bind(&payload); err != nil {
		return combos.Combo{}, core.NewInvalidRequestError("invalid combo payload", err)
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	return combos.Combo{
		ID:          name,
		Name:        name,
		Description: payload.Description,
		Models:      payload.Models,
		Enabled:     enabled,
		Source:      combos.SourceAdmin,
	}, nil
}
