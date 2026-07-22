package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

const liveSubBufferSize = 64

func (h *Handler) AuditLogStream(c *echo.Context) error {
	if h.auditLogger == nil {
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

	sub := h.auditLogger.SubscribeLive(liveSubBufferSize)
	defer h.auditLogger.UnsubscribeLive(sub.ID)

	ctx := c.Request().Context()
	enc := json.NewEncoder(resp)

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry, ok := <-sub.Entries:
			if !ok {
				return nil
			}
			if _, err := fmt.Fprint(resp, "data: "); err != nil {
				return nil
			}
			if err := enc.Encode(entry); err != nil {
				continue
			}
			if _, err := fmt.Fprint(resp, "\n"); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}
