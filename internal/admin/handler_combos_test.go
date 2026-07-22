package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/internal/core"
	"aurora/internal/model_combinations"
)

func TestUpdateComboUsesRouteTargetIDForRename(t *testing.T) {
	service := newAdminComboService(t, nil)
	if err := service.Upsert(context.Background(), combos.Combo{Name: "old", Models: []string{"a/model", "b/model"}, Enabled: true}); err != nil {
		t.Fatalf("seed combo: %v", err)
	}
	h := NewHandler(nil, nil, WithCombos(service))
	e := echo.New()

	req := httptest.NewRequest(http.MethodPut, "/admin/api/v1/combos/old", bytes.NewBufferString(`{"name":"new","models":["c/model","d/model"],"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "old"}})

	if err := h.UpdateCombo(ctx); err != nil {
		t.Fatalf("UpdateCombo() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if _, ok := service.Get("old"); ok {
		t.Fatalf("old combo still exists after rename")
	}
	renamed, ok := service.Get("new")
	if !ok {
		t.Fatalf("renamed combo not found")
	}
	if renamed.ID != "old" {
		t.Fatalf("renamed ID = %q, want old", renamed.ID)
	}
}

func TestUpdateComboRejectsStaticRouteTargetEvenWithDifferentBodyName(t *testing.T) {
	service := newAdminComboService(t, []combos.Combo{{ID: "static", Name: "static", Models: []string{"a/model", "b/model"}, Enabled: true, Source: combos.SourceStatic}})
	h := NewHandler(nil, nil, WithCombos(service))
	e := echo.New()

	req := httptest.NewRequest(http.MethodPut, "/admin/api/v1/combos/static", bytes.NewBufferString(`{"name":"admin","models":["c/model","d/model"],"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "static"}})

	if err := h.UpdateCombo(ctx); err != nil {
		t.Fatalf("UpdateCombo() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if _, ok := service.Get("admin"); ok {
		t.Fatalf("admin combo was created while updating static target")
	}
	if static, ok := service.Get("static"); !ok || static.Source != combos.SourceStatic {
		t.Fatalf("static combo missing or mutated: %#v", static)
	}
}

func newAdminComboService(t *testing.T, static []combos.Combo) *combos.Service {
	t.Helper()
	catalog := adminComboTestCatalog{supported: map[string]struct{}{
		"a/model": {},
		"b/model": {},
		"c/model": {},
		"d/model": {},
	}}
	service, err := combos.NewServiceWithStatic(combos.NewMemoryStore(nil), catalog, nil, static)
	if err != nil {
		t.Fatalf("new combo service: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh combo service: %v", err)
	}
	return service
}

type adminComboTestCatalog struct {
	supported map[string]struct{}
}

func (c adminComboTestCatalog) Supports(model string) bool {
	_, ok := c.supported[model]
	return ok
}

func (c adminComboTestCatalog) LookupModel(model string) (*core.Model, bool) {
	if !c.Supports(model) {
		return nil, false
	}
	return &core.Model{ID: model}, true
}
