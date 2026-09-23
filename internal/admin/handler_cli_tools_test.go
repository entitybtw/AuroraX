package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/internal/command_line_tools"
)

func TestListCLIToolsReturnsToolsAndPresets(t *testing.T) {
	service := clitools.NewService(false, nil)
	h := NewHandler(nil, nil, WithCLITools(service))
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/cli-tools", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h.ListCLITools(ctx); err != nil {
		t.Fatalf("ListCLITools() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload struct {
		Tools []struct {
			ID string `json:"id"`
		} `json:"tools"`
		Presets []struct {
			ID     string `json:"id"`
			ToolID string `json:"tool_id"`
		} `json:"presets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Tools) == 0 {
		t.Fatal("expected tools in response")
	}
	if len(payload.Presets) == 0 {
		t.Fatal("expected presets in response")
	}
	for _, preset := range payload.Presets {
		if preset.ID == "" || preset.ToolID == "" {
			t.Fatalf("preset missing id or tool_id: %+v", preset)
		}
	}
}

func TestListCLIToolsWithoutServiceReturnsEmptyPresets(t *testing.T) {
	h := NewHandler(nil, nil)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/cli-tools", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h.ListCLITools(ctx); err != nil {
		t.Fatalf("ListCLITools() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["presets"]; !ok {
		t.Fatal("expected presets key in response")
	}
}
