package admin

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"aurora/internal/command_line_tools"
	"aurora/internal/core"
)

func (h *Handler) ListCLITools(c *echo.Context) error {
	if h.cliTools == nil {
		return c.JSON(http.StatusOK, map[string]any{"tools": []any{}})
	}
	return c.JSON(http.StatusOK, map[string]any{"tools": h.cliTools.ListTools()})
}

func (h *Handler) GetCLITool(c *echo.Context) error {
	if h.cliTools == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "CLI tools are not enabled", nil))
	}
	tool, ok := h.cliTools.GetTool(c.Param("tool"))
	if !ok {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "CLI tool not found", nil))
	}
	return c.JSON(http.StatusOK, tool)
}

func (h *Handler) PreviewCLITool(c *echo.Context) error {
	if h.cliTools == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "CLI tools are not enabled", nil))
	}
	var req clitools.PreviewRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid CLI tool preview payload", err))
	}
	resp, err := h.cliTools.Preview(c.Param("tool"), req)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), err))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ApplyCLITool(c *echo.Context) error {
	if h.cliTools == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "CLI tools are not enabled", nil))
	}
	var req clitools.PreviewRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid CLI tool apply payload", err))
	}
	resp, err := h.cliTools.Apply(c.Param("tool"), req)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), err))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ResetCLITool(c *echo.Context) error {
	return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotImplemented, "CLI tool reset is not implemented yet", nil))
}
