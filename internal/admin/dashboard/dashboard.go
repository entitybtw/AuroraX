package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v5"
)

//go:embed dist
var embeddedDist embed.FS

const basePathPlaceholder = "__AURORA_BASE_PATH__"

type Handler struct {
	basePath string
	dist     fs.FS
	index    []byte
}

func New() (*Handler, error) {
	return NewWithBasePath("/")
}

func NewWithBasePath(basePath string) (*Handler, error) {
	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, fmt.Errorf("react dashboard: open embedded dist: %w", err)
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, fmt.Errorf("react dashboard: read index.html: %w", err)
	}
	return &Handler{
		basePath: normalizeBasePath(basePath),
		dist:     dist,
		index:    index,
	}, nil
}

func (h *Handler) Index(c *echo.Context) error {
	body := strings.ReplaceAll(string(h.index), basePathPlaceholder, h.basePath)
	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.HTMLBlob(http.StatusOK, []byte(body))
}

func (h *Handler) Static(c *echo.Context) error {
	name := staticAssetName(c.Request().URL.Path)
	if name == "" {
		return echo.NewHTTPError(http.StatusNotFound, "asset not found")
	}
	data, err := fs.ReadFile(h.dist, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "asset not found")
	}
	if strings.HasPrefix(name, "assets/") {
		c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Response().Header().Set("Cache-Control", "no-cache")
	}
	return c.Blob(http.StatusOK, contentType(name, data), data)
}

func staticAssetName(urlPath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(urlPath))
	name := strings.TrimPrefix(cleaned, "/admin/static/")
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." || strings.HasPrefix(name, "../") || name == ".." {
		return ""
	}
	return name
}

func contentType(name string, data []byte) string {
	if typ := mime.TypeByExtension(path.Ext(name)); typ != "" {
		return typ
	}
	return http.DetectContentType(data)
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "." || basePath == "/" {
		return "/"
	}
	basePath = path.Clean("/" + basePath)
	if basePath == "." {
		return "/"
	}
	return basePath
}
