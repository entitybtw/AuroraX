package admin

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"gopkg.in/yaml.v3"

	"aurora/internal/core"
	"aurora/internal/model_data"
	"aurora/internal/providers"
)

type modelPricingView struct {
	Selector         string             `json:"selector"`
	OverrideSelector string             `json:"override_selector"`
	ProviderName     string             `json:"provider_name"`
	ProviderType     string             `json:"provider_type"`
	ModelID          string             `json:"model_id"`
	DisplayName      string             `json:"display_name,omitempty"`
	Source           string             `json:"source"`
	HasOverride      bool               `json:"has_override"`
	OverriddenFields []string           `json:"overridden_fields,omitempty"`
	BasePricing      *core.ModelPricing `json:"base_pricing,omitempty"`
	OverridePricing  *core.ModelPricing `json:"override_pricing,omitempty"`
	EffectivePricing *core.ModelPricing `json:"effective_pricing,omitempty"`
}

type upsertModelPricingRequest struct {
	Pricing core.ModelPricing `json:"pricing"`
}

type importModelPricingRequest struct {
	Format  string `json:"format"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
}

type modelPricingImportResponse struct {
	Mode                string   `json:"mode"`
	Applied             bool     `json:"applied"`
	BackupName          string   `json:"backup_name,omitempty"`
	CurrentOverrideKeys []string `json:"current_override_keys"`
	IncomingKeys        []string `json:"incoming_keys"`
	AddedKeys           []string `json:"added_keys"`
	ChangedKeys         []string `json:"changed_keys"`
	RemovedKeys         []string `json:"removed_keys"`
}

type modelPricingBackupView struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
}

type restoreModelPricingBackupResponse struct {
	Message    string `json:"message"`
	BackupName string `json:"backup_name"`
}

func (h *Handler) ListModelPricing(c *echo.Context) error {
	views, err := h.modelPricingViews()
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, views)
}

func (h *Handler) GetModelPricing(c *echo.Context) error {
	selector, err := decodeModelOverridePathSelector(c.Param("selector"))
	if err != nil {
		return handleError(c, err)
	}
	views, err := h.modelPricingViews()
	if err != nil {
		return handleError(c, err)
	}
	for _, view := range views {
		if view.Selector == selector {
			return c.JSON(http.StatusOK, view)
		}
	}
	return handleError(c, core.NewNotFoundError("model pricing not found: "+selector))
}

func (h *Handler) UpsertModelPricing(c *echo.Context) error {
	selector, err := decodeModelOverridePathSelector(c.Param("selector"))
	if err != nil {
		return handleError(c, err)
	}
	var req upsertModelPricingRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	if err := validatePricingOverride(&req.Pricing); err != nil {
		return handleError(c, err)
	}
	view, err := h.modelPricingView(selector)
	if err != nil {
		return handleError(c, err)
	}
	if err := h.savePricingOverride(view.OverrideSelector, &req.Pricing); err != nil {
		return handleError(c, err)
	}
	updated, err := h.modelPricingView(selector)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteModelPricing(c *echo.Context) error {
	selector, err := decodeModelOverridePathSelector(c.Param("selector"))
	if err != nil {
		return handleError(c, err)
	}
	view, err := h.modelPricingView(selector)
	if err != nil {
		return handleError(c, err)
	}
	if err := h.deletePricingOverride(view.OverrideSelector); err != nil {
		return handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ExportModelPricing(c *echo.Context) error {
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return handleError(c, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			raw = []byte("models: {}\nprovider_models: {}\n")
		} else {
			return handleError(c, core.NewProviderError("model_pricing", http.StatusBadGateway, "export pricing overrides: "+err.Error(), err))
		}
	}
	return c.Blob(http.StatusOK, "application/x-yaml", raw)
}

func (h *Handler) ImportModelPricing(c *echo.Context) error {
	var req importModelPricingRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	resp, err := h.importPricingOverrides(req)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListModelPricingBackups(c *echo.Context) error {
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return handleError(c, err)
	}
	entries, err := os.ReadDir(modelPricingBackupDir(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return c.JSON(http.StatusOK, []modelPricingBackupView{})
		}
		return handleError(c, core.NewProviderError("model_pricing", http.StatusBadGateway, "list pricing backups: "+err.Error(), err))
	}
	views := make([]modelPricingBackupView, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		views = append(views, modelPricingBackupView{Name: entry.Name(), Path: filepath.Join(modelPricingBackupDir(path), entry.Name()), SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ModifiedAt.After(views[j].ModifiedAt) })
	return c.JSON(http.StatusOK, views)
}

func (h *Handler) RestoreModelPricingBackup(c *echo.Context) error {
	name, err := decodeAliasPathName(c.Param("name"))
	if err != nil {
		return handleError(c, err)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || !strings.HasSuffix(name, ".yaml") {
		return handleError(c, core.NewInvalidRequestError("invalid backup name", nil))
	}
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return handleError(c, err)
	}
	source := filepath.Join(modelPricingBackupDir(path), name)
	raw, err := os.ReadFile(source)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return handleError(c, core.NewNotFoundError("pricing backup not found: "+name))
		}
		return handleError(c, core.NewProviderError("model_pricing", http.StatusBadGateway, "read pricing backup: "+err.Error(), err))
	}
	var parsed modeldata.UserOverrides
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return handleError(c, core.NewInvalidRequestError("backup is not valid pricing YAML: "+err.Error(), err))
	}
	if _, err := h.writePricingOverrides(path, &parsed, true); err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, restoreModelPricingBackupResponse{Message: "pricing backup restored", BackupName: name})
}

func (h *Handler) modelPricingViews() ([]modelPricingView, error) {
	if h.registry == nil {
		return nil, featureUnavailableError("model pricing controls are unavailable")
	}
	overrides, err := h.loadPricingOverrides()
	if err != nil {
		return nil, err
	}
	baseList := h.registry.BaseModelList()
	models := h.registry.ListModelsWithProvider()
	views := make([]modelPricingView, 0, len(models))
	for _, item := range models {
		views = append(views, buildModelPricingView(item, baseList, overrides))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Selector < views[j].Selector })
	return views, nil
}

func (h *Handler) modelPricingView(selector string) (modelPricingView, error) {
	views, err := h.modelPricingViews()
	if err != nil {
		return modelPricingView{}, err
	}
	for _, view := range views {
		if view.Selector == selector {
			return view, nil
		}
	}
	return modelPricingView{}, core.NewNotFoundError("model pricing not found: " + selector)
}

func buildModelPricingView(item providers.ModelWithProvider, baseList *modeldata.ModelList, overrides *modeldata.UserOverrides) modelPricingView {
	selector := strings.TrimSpace(item.Selector)
	modelID := strings.TrimSpace(item.Model.ID)
	providerName := strings.TrimSpace(item.ProviderName)
	providerType := strings.TrimSpace(item.ProviderType)
	overrideSelector := providerType + "/" + modelID
	if providerType == "" {
		overrideSelector = selector
	}

	var basePricing *core.ModelPricing
	if baseList != nil {
		if meta := modeldata.Resolve(baseList, providerType, modelID); meta != nil && meta.Pricing != nil {
			basePricing = meta.Pricing.Clone()
		}
	}

	var overridePricing *core.ModelPricing
	if overrides != nil && overrides.ProviderModels != nil {
		if override := overrides.ProviderModels[overrideSelector]; override.Pricing != nil {
			overridePricing = override.Pricing.Clone()
		}
	}

	var effectivePricing *core.ModelPricing
	if item.Model.Metadata != nil && item.Model.Metadata.Pricing != nil {
		effectivePricing = item.Model.Metadata.Pricing.Clone()
	}

	source := "missing"
	if effectivePricing != nil {
		source = "registry"
	}
	if overridePricing != nil {
		source = "user_override"
	}

	displayName := ""
	if item.Model.Metadata != nil {
		displayName = strings.TrimSpace(item.Model.Metadata.DisplayName)
	}
	return modelPricingView{
		Selector:         selector,
		OverrideSelector: overrideSelector,
		ProviderName:     providerName,
		ProviderType:     providerType,
		ModelID:          modelID,
		DisplayName:      displayName,
		Source:           source,
		HasOverride:      overridePricing != nil,
		OverriddenFields: pricingOverrideFields(overridePricing),
		BasePricing:      basePricing,
		OverridePricing:  overridePricing,
		EffectivePricing: effectivePricing,
	}
}

func (h *Handler) loadPricingOverrides() (*modeldata.UserOverrides, error) {
	path := strings.TrimSpace(h.registry.UserOverridesPath())
	if path == "" {
		return nil, nil
	}
	overrides, err := modeldata.LoadUserOverrides(path)
	if err != nil {
		return nil, core.NewProviderError("model_pricing", http.StatusBadGateway, "load pricing overrides: "+err.Error(), err)
	}
	return overrides, nil
}

func (h *Handler) modelPricingOverridesPath() (string, error) {
	if h.registry == nil {
		return "", featureUnavailableError("model pricing controls are unavailable")
	}
	path := strings.TrimSpace(h.registry.UserOverridesPath())
	if path == "" {
		return "", featureUnavailableError("model pricing overrides path is not configured")
	}
	return path, nil
}

func (h *Handler) importPricingOverrides(req importModelPricingRequest) (modelPricingImportResponse, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "dry_run"
	}
	if mode != "dry_run" && mode != "merge" && mode != "replace" {
		return modelPricingImportResponse{}, core.NewInvalidRequestError("import mode must be dry_run, merge, or replace", nil)
	}
	var incoming modeldata.UserOverrides
	if err := yaml.Unmarshal([]byte(req.Content), &incoming); err != nil {
		return modelPricingImportResponse{}, core.NewInvalidRequestError("invalid pricing import YAML: "+err.Error(), err)
	}
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return modelPricingImportResponse{}, err
	}
	current, err := modeldata.LoadUserOverrides(path)
	if err != nil {
		return modelPricingImportResponse{}, core.NewProviderError("model_pricing", http.StatusBadGateway, "load pricing overrides: "+err.Error(), err)
	}
	merged := &incoming
	if mode == "merge" || mode == "dry_run" {
		merged = mergeUserOverrides(current, &incoming)
	}
	resp := pricingImportDiff(current, &incoming, merged, mode)
	if mode == "dry_run" {
		return resp, nil
	}
	backupName, err := h.writePricingOverrides(path, merged, true)
	if err != nil {
		return modelPricingImportResponse{}, err
	}
	resp.Applied = true
	resp.BackupName = backupName
	return resp, nil
}

func (h *Handler) writePricingOverrides(path string, overrides *modeldata.UserOverrides, backup bool) (string, error) {
	h.pricingMu.Lock()
	defer h.pricingMu.Unlock()
	backupName := ""
	if backup {
		name, err := createPricingBackup(path)
		if err != nil {
			return "", core.NewProviderError("model_pricing", http.StatusBadGateway, "backup pricing overrides: "+err.Error(), err)
		}
		backupName = name
	}
	if err := modeldata.SaveUserOverrides(path, overrides); err != nil {
		return "", core.NewProviderError("model_pricing", http.StatusBadGateway, "save pricing overrides: "+err.Error(), err)
	}
	if err := h.registry.ReloadUserOverrides(); err != nil {
		return "", core.NewProviderError("model_pricing", http.StatusBadGateway, "reload pricing overrides: "+err.Error(), err)
	}
	return backupName, nil
}

func (h *Handler) savePricingOverride(selector string, pricing *core.ModelPricing) error {
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return err
	}
	overrides, err := modeldata.LoadUserOverrides(path)
	if err != nil {
		return core.NewProviderError("model_pricing", http.StatusBadGateway, "load pricing overrides: "+err.Error(), err)
	}
	overrides = modeldata.UpsertProviderModelPricingOverride(overrides, selector, pricing)
	_, err = h.writePricingOverrides(path, overrides, true)
	return err
}

func (h *Handler) deletePricingOverride(selector string) error {
	path, err := h.modelPricingOverridesPath()
	if err != nil {
		return err
	}
	overrides, err := modeldata.LoadUserOverrides(path)
	if err != nil {
		return core.NewProviderError("model_pricing", http.StatusBadGateway, "load pricing overrides: "+err.Error(), err)
	}
	overrides = modeldata.DeleteProviderModelPricingOverride(overrides, selector)
	_, err = h.writePricingOverrides(path, overrides, true)
	return err
}

func validatePricingOverride(pricing *core.ModelPricing) error {
	if pricing == nil {
		return core.NewInvalidRequestError("pricing is required", nil)
	}
	if strings.TrimSpace(pricing.Currency) == "" {
		return core.NewInvalidRequestError("pricing currency is required", nil)
	}
	for _, field := range pricingFloatFields(pricing) {
		if field.value == nil {
			continue
		}
		if math.IsNaN(*field.value) || math.IsInf(*field.value, 0) || *field.value < 0 {
			return core.NewInvalidRequestError(fmt.Sprintf("pricing %s must be a finite value >= 0", field.name), nil)
		}
	}
	for i, tier := range pricing.Tiers {
		for _, field := range []struct {
			name  string
			value *float64
		}{
			{fmt.Sprintf("tiers[%d].up_to_mtok", i), tier.UpToMtok},
			{fmt.Sprintf("tiers[%d].input_per_mtok", i), tier.InputPerMtok},
			{fmt.Sprintf("tiers[%d].output_per_mtok", i), tier.OutputPerMtok},
		} {
			if field.value != nil && (math.IsNaN(*field.value) || math.IsInf(*field.value, 0) || *field.value < 0) {
				return core.NewInvalidRequestError(fmt.Sprintf("pricing %s must be a finite value >= 0", field.name), nil)
			}
		}
	}
	return nil
}

func pricingOverrideFields(pricing *core.ModelPricing) []string {
	if pricing == nil {
		return nil
	}
	fields := make([]string, 0, 8)
	if strings.TrimSpace(pricing.Currency) != "" {
		fields = append(fields, "currency")
	}
	for _, field := range pricingFloatFields(pricing) {
		if field.value != nil {
			fields = append(fields, field.name)
		}
	}
	if len(pricing.Tiers) > 0 {
		fields = append(fields, "tiers")
	}
	return fields
}

func pricingFloatFields(pricing *core.ModelPricing) []struct {
	name  string
	value *float64
} {
	return []struct {
		name  string
		value *float64
	}{
		{"input_per_mtok", pricing.InputPerMtok},
		{"output_per_mtok", pricing.OutputPerMtok},
		{"cached_input_per_mtok", pricing.CachedInputPerMtok},
		{"cache_write_per_mtok", pricing.CacheWritePerMtok},
		{"reasoning_output_per_mtok", pricing.ReasoningOutputPerMtok},
		{"batch_input_per_mtok", pricing.BatchInputPerMtok},
		{"batch_output_per_mtok", pricing.BatchOutputPerMtok},
		{"audio_input_per_mtok", pricing.AudioInputPerMtok},
		{"audio_output_per_mtok", pricing.AudioOutputPerMtok},
		{"per_image", pricing.PerImage},
		{"input_per_image", pricing.InputPerImage},
		{"per_second_input", pricing.PerSecondInput},
		{"per_second_output", pricing.PerSecondOutput},
		{"per_character_input", pricing.PerCharacterInput},
		{"per_request", pricing.PerRequest},
		{"per_page", pricing.PerPage},
	}
}

func modelPricingBackupDir(path string) string {
	return filepath.Join(filepath.Dir(path), "user_pricing.backups")
}

func createPricingBackup(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if err := os.MkdirAll(modelPricingBackupDir(path), 0o755); err != nil {
		return "", err
	}
	name := "user_pricing-" + time.Now().UTC().Format("20060102-150405") + ".yaml"
	return name, os.WriteFile(filepath.Join(modelPricingBackupDir(path), name), raw, 0o644)
}

func mergeUserOverrides(current, incoming *modeldata.UserOverrides) *modeldata.UserOverrides {
	out := &modeldata.UserOverrides{Models: map[string]modeldata.ModelOverride{}, ProviderModels: map[string]modeldata.ProviderModelOverride{}}
	if current != nil {
		for key, value := range current.Models {
			out.Models[key] = value
		}
		for key, value := range current.ProviderModels {
			out.ProviderModels[key] = value
		}
	}
	if incoming != nil {
		for key, value := range incoming.Models {
			out.Models[key] = value
		}
		for key, value := range incoming.ProviderModels {
			out.ProviderModels[key] = value
		}
	}
	return out
}

func pricingImportDiff(current, incoming, final *modeldata.UserOverrides, mode string) modelPricingImportResponse {
	currentKeys := pricingOverrideKeys(current)
	incomingKeys := pricingOverrideKeys(incoming)
	finalKeys := pricingOverrideKeys(final)
	currentSet := stringSet(currentKeys)
	incomingSet := stringSet(incomingKeys)
	finalSet := stringSet(finalKeys)
	added := make([]string, 0)
	changed := make([]string, 0)
	removed := make([]string, 0)
	for key := range finalSet {
		if _, ok := currentSet[key]; !ok {
			added = append(added, key)
			continue
		}
		if _, ok := incomingSet[key]; ok {
			changed = append(changed, key)
		}
	}
	for key := range currentSet {
		if _, ok := finalSet[key]; !ok {
			removed = append(removed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(changed)
	sort.Strings(removed)
	return modelPricingImportResponse{Mode: mode, CurrentOverrideKeys: currentKeys, IncomingKeys: incomingKeys, AddedKeys: added, ChangedKeys: changed, RemovedKeys: removed}
}

func pricingOverrideKeys(overrides *modeldata.UserOverrides) []string {
	if overrides == nil || len(overrides.ProviderModels) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(overrides.ProviderModels))
	for key, value := range overrides.ProviderModels {
		if value.Pricing != nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
