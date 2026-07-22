package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
)

const defaultConsoleRecentLimit = 200

func (h *Handler) ConsoleRecent(c *echo.Context) error {
	if h.console == nil {
		return c.JSON(http.StatusOK, map[string]any{"events": []any{}, "total": 0})
	}
	limit := defaultConsoleRecentLimit
	if raw := c.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = min(parsed, 1000)
		}
	}
	offset := 0
	if raw := c.QueryParam("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"events": h.console.Recent(limit, offset),
		"total":  h.console.LenRecent(),
	})
}

func (h *Handler) ConsoleStream(c *echo.Context) error {
	if h.console == nil {
		return c.JSON(http.StatusOK, struct{}{})
	}

	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := resp.(http.Flusher)
	if !ok {
		return c.JSON(http.StatusInternalServerError, struct{}{})
	}

	sub := h.console.Subscribe(liveSubBufferSize)
	defer h.console.Unsubscribe(sub.ID)

	ctx := c.Request().Context()
	enc := json.NewEncoder(resp)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-sub.Events:
			if !ok {
				return nil
			}
			if _, err := fmt.Fprint(resp, "data: "); err != nil {
				return nil
			}
			if err := enc.Encode(event); err != nil {
				continue
			}
			if _, err := fmt.Fprint(resp, "\n"); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}
