package sessionhub

import (
	"fmt"
	"sort"

	"github.com/labstack/echo/v5"
)

// RegisterSessionHubRoutes mounts session hub admin routes on the given group.
func RegisterSessionHubRoutes(g interface {
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
}, hub *Hub) {

	g.GET("/sessionhub/status", func(c *echo.Context) error {
		stats := hub.Stats()
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data":   stats,
		})
	})

	g.GET("/sessionhub/providers", func(c *echo.Context) error {
		cfg := hub.Config()
		providers := make([]map[string]interface{}, 0, len(cfg.Providers))
		for name, rule := range cfg.Providers {
			providers = append(providers, map[string]interface{}{
				"name":    name,
				"enabled": rule.Enabled,
				"headers": len(rule.Headers),
			})
		}
		sort.Slice(providers, func(i, j int) bool {
			return providers[i]["name"].(string) < providers[j]["name"].(string)
		})
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data":   providers,
		})
	})

	g.POST("/sessionhub/providers", func(c *echo.Context) error {
		var req struct {
			Name string       `json:"name"`
			Rule ProviderRule `json:"rule"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "invalid JSON: " + err.Error()})
		}
		if req.Name == "" {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "name is required"})
		}
		if err := ValidateRule(req.Name, req.Rule); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": err.Error()})
		}
		hub.SetProviderRule(req.Name, req.Rule)
		return c.JSON(200, map[string]interface{}{"status": "ok", "data": map[string]string{"name": req.Name, "status": "created"}})
	})

	g.GET("/sessionhub/providers/:name", func(c *echo.Context) error {
		name := c.Param("name")
		rule, ok := hub.GetProviderRule(name)
		if !ok {
			return c.JSON(404, map[string]interface{}{"status": "error", "error": "provider not found"})
		}
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"name": name, "rule": rule},
		})
	})

	g.PUT("/sessionhub/providers/:name", func(c *echo.Context) error {
		name := c.Param("name")
		var rule ProviderRule
		if err := c.Bind(&rule); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "invalid JSON: " + err.Error()})
		}
		if err := ValidateRule(name, rule); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": err.Error()})
		}
		hub.SetProviderRule(name, rule)
		return c.JSON(200, map[string]interface{}{"status": "ok", "data": map[string]string{"name": name, "status": "updated"}})
	})

	g.DELETE("/sessionhub/providers/:name", func(c *echo.Context) error {
		name := c.Param("name")
		hub.DeleteProviderRule(name)
		return c.JSON(200, map[string]interface{}{"status": "ok", "data": map[string]string{"name": name, "status": "deleted"}})
	})

	g.GET("/sessionhub/mappings", func(c *echo.Context) error {
		entries := hub.Store().List()
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"mappings": entries,
				"total":    len(entries),
			},
		})
	})

	g.DELETE("/sessionhub/mappings", func(c *echo.Context) error {
		hub.Store().Clear()
		return c.JSON(200, map[string]interface{}{"status": "ok", "data": map[string]string{"status": "cleared"}})
	})

	g.DELETE("/sessionhub/mappings/:provider", func(c *echo.Context) error {
		provider := c.Param("provider")
		entries := hub.Store().List()
		count := 0
		for _, e := range entries {
			if e.Provider == provider {
				hub.Store().Delete(provider, e.InboundValue)
				count++
			}
		}
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data":   map[string]interface{}{"provider": provider, "cleared": count},
		})
	})

	g.POST("/sessionhub/reload", func(c *echo.Context) error {
		var req struct {
			Config *HubConfig `json:"config"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "invalid JSON: " + err.Error()})
		}
		if req.Config == nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "config is required"})
		}
		hub.Reload(req.Config)
		return c.JSON(200, map[string]interface{}{"status": "ok", "data": map[string]string{"status": "reloaded"}})
	})

	g.POST("/sessionhub/apply", func(c *echo.Context) error {
		var req struct {
			Provider string            `json:"provider"`
			Headers  map[string]string `json:"headers"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "invalid JSON"})
		}
		if req.Provider == "" {
			return c.JSON(400, map[string]interface{}{"status": "error", "error": "provider is required"})
		}
		h := make(map[string][]string)
		for k, v := range req.Headers {
			h[fmt.Sprintf("%s", k)] = []string{v}
		}
		result := hub.Apply(h, req.Provider)
		return c.JSON(200, map[string]interface{}{
			"status": "ok",
			"data":   result,
		})
	})
}
