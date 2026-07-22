package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func CapabilityGateMiddleware(capabilities map[string]bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			capability, required := RequiredCapabilityForRoute(c.Request().URL.Path)
			if !required || capabilities[string(capability)] {
				return next(c)
			}
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"type":       "feature_restricted",
					"message":    "this endpoint is not available in this build",
					"code":       "feature_not_available",
					"capability": string(capability),
				},
			})
		}
	}
}
