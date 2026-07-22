package combos

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurora/configuration"
	"aurora/internal/core"
)

var validComboName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

const (
	maxNameLength        = 128
	maxDescriptionLength = 2048
	maxModelSelectorLen  = 256
	maxModelsPerCombo    = 20
)

type Service struct {
	store      Store
	catalog    Catalog
	downstream DownstreamResolver

	static []Combo

	mu       sync.RWMutex
	byName   map[string]Combo
	ordered  []string
	readonly map[string]bool
}

func NewService(store Store, catalog Catalog, downstream DownstreamResolver) (*Service, error) {
	return NewServiceWithStatic(store, catalog, downstream, nil)
}

func NewServiceWithStatic(store Store, catalog Catalog, downstream DownstreamResolver, static []Combo) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if catalog == nil {
		return nil, fmt.Errorf("catalog is required")
	}
	staticCombos := make([]Combo, 0, len(static))
	for _, combo := range static {
		staticCombos = append(staticCombos, cloneCombo(combo))
	}
	return &Service{store: store, catalog: catalog, downstream: downstream, static: staticCombos}, nil
}

func CombosFromConfig(definitions []config.ComboDefinition) []Combo {
	now := time.Now().UTC()
	out := make([]Combo, 0, len(definitions))
	for _, def := range definitions {
		enabled := true
		if def.Enabled != nil {
			enabled = *def.Enabled
		}
		name := strings.TrimSpace(def.Name)
		out = append(out, Combo{
			ID:          name,
			Name:        name,
			Description: strings.TrimSpace(def.Description),
			Models:      normalizeStrings(def.Models),
			Enabled:     enabled,
			Source:      SourceStatic,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return out
}

func (s *Service) Refresh(ctx context.Context) error {
	adminCombos, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list combos: %w", err)
	}
	allCombos := make([]Combo, 0, len(s.static)+len(adminCombos))
	allCombos = append(allCombos, s.static...)
	allCombos = append(allCombos, adminCombos...)
	nextByName := make(map[string]Combo, len(allCombos))
	nextReadonly := make(map[string]bool, len(allCombos))
	nextOrdered := make([]string, 0, len(allCombos))
	for _, combo := range allCombos {
		normalized, err := s.normalize(combo)
		if err != nil {
			return fmt.Errorf("load combo %q: %w", combo.Name, err)
		}
		if _, exists := nextByName[normalized.Name]; exists {
			return fmt.Errorf("duplicate combo name %q", normalized.Name)
		}
		nextByName[normalized.Name] = normalized
		nextReadonly[normalized.Name] = normalized.Source == SourceStatic
		nextOrdered = append(nextOrdered, normalized.Name)
	}
	sort.Strings(nextOrdered)
	s.mu.Lock()
	s.byName = nextByName
	s.readonly = nextReadonly
	s.ordered = nextOrdered
	s.mu.Unlock()
	return nil
}

func (s *Service) List() []Combo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Combo, 0, len(s.ordered))
	for _, name := range s.ordered {
		out = append(out, cloneCombo(s.byName[name]))
	}
	return out
}

func (s *Service) ListViews() []View {
	combos := s.List()
	views := make([]View, 0, len(combos))
	for _, combo := range combos {
		views = append(views, s.Validate(combo))
	}
	return views
}

func (s *Service) Get(name string) (*Combo, bool) {
	if s == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	s.mu.RLock()
	defer s.mu.RUnlock()
	combo, ok := s.byName[name]
	if !ok {
		return nil, false
	}
	copy := cloneCombo(combo)
	return &copy, true
}

func (s *Service) ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	if !requested.ExplicitProvider {
		if combo, ok := s.Get(requested.Model); ok && combo.Enabled {
			if len(combo.Models) == 0 {
				return core.ModelSelector{}, false, fmt.Errorf("combo %q has no models", combo.Name)
			}
			selector, err := core.ParseModelSelector(combo.Models[0], "")
			return selector, true, err
		}
	}
	if s.downstream != nil {
		return s.downstream.ResolveModel(requested)
	}
	selector, err := requested.Normalize()
	return selector, false, err
}

func (s *Service) FallbacksFor(requestedModel string) []core.ModelSelector {
	if s == nil {
		return nil
	}
	combo, ok := s.Get(requestedModel)
	if !ok || !combo.Enabled || len(combo.Models) < 2 {
		return nil
	}
	out := make([]core.ModelSelector, 0, len(combo.Models)-1)
	for _, model := range combo.Models[1:] {
		selector, err := core.ParseModelSelector(model, "")
		if err == nil && s.catalog.Supports(selector.QualifiedModel()) {
			out = append(out, selector)
		}
	}
	return out
}

func (s *Service) ExposedModels() []core.Model {
	combos := s.List()
	out := make([]core.Model, 0, len(combos))
	for _, combo := range combos {
		if !combo.Enabled || len(combo.Models) == 0 {
			continue
		}
		primary, err := core.ParseModelSelector(combo.Models[0], "")
		if err != nil {
			continue
		}
		base, ok := s.catalog.LookupModel(primary.QualifiedModel())
		if !ok || base == nil {
			continue
		}
		cloned := *base
		cloned.ID = combo.Name
		cloned.OwnedBy = "aurora-combo"
		if cloned.Metadata != nil {
			cloned.Metadata = cloned.Metadata.Clone()
		}
		out = append(out, cloned)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) Upsert(ctx context.Context, combo Combo) error {
	normalized, err := s.normalize(combo)
	if err != nil {
		return err
	}
	if normalized.Source == "" {
		normalized.Source = SourceAdmin
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now().UTC()
	}
	normalized.UpdatedAt = time.Now().UTC()
	view := s.Validate(normalized)
	if !view.Valid {
		return errors.New(strings.Join(view.Errors, "; "))
	}
	if existing, ok := s.Get(normalized.Name); ok && existing.Source == SourceStatic {
		return fmt.Errorf("static combo %q cannot be modified through admin API", normalized.Name)
	}
	if err := s.store.Upsert(ctx, normalized); err != nil {
		return fmt.Errorf("upsert combo: %w", err)
	}
	return s.Refresh(ctx)
}

func (s *Service) Delete(ctx context.Context, idOrName string) error {
	if existing, ok := s.Get(idOrName); ok && existing.Source == SourceStatic {
		return fmt.Errorf("static combo %q cannot be deleted through admin API", existing.Name)
	}
	if err := s.store.Delete(ctx, idOrName); err != nil {
		return fmt.Errorf("delete combo: %w", err)
	}
	return s.Refresh(ctx)
}

func (s *Service) Validate(combo Combo) View {
	normalized, err := s.normalize(combo)
	view := View{Combo: normalized, Readonly: normalized.Source == SourceStatic}
	if err != nil {
		view.Errors = append(view.Errors, err.Error())
		return view
	}
	if len(normalized.Models) > 0 {
		view.Primary = normalized.Models[0]
	}
	if len(normalized.Models) > 1 {
		view.Fallbacks = append([]string(nil), normalized.Models[1:]...)
	}
	seen := make(map[string]struct{}, len(normalized.Models))
	for _, model := range normalized.Models {
		selector, err := core.ParseModelSelector(model, "")
		if err != nil {
			view.Errors = append(view.Errors, "invalid model selector "+model+": "+err.Error())
			continue
		}
		qualified := selector.QualifiedModel()
		if _, exists := seen[qualified]; exists {
			view.Errors = append(view.Errors, "duplicate model selector: "+qualified)
		}
		seen[qualified] = struct{}{}
		if _, ok := s.Get(qualified); ok {
			view.Errors = append(view.Errors, "nested combos are not supported: "+qualified)
		}
		if !s.catalog.Supports(qualified) {
			view.Errors = append(view.Errors, "model not currently available: "+qualified)
		}
	}
	view.Valid = len(view.Errors) == 0
	return view
}

func (s *Service) normalize(combo Combo) (Combo, error) {
	combo.Name = strings.TrimSpace(combo.Name)
	combo.ID = strings.TrimSpace(combo.ID)
	combo.Description = strings.TrimSpace(combo.Description)
	combo.Source = strings.TrimSpace(combo.Source)
	combo.Models = normalizeStrings(combo.Models)
	if combo.ID == "" {
		combo.ID = combo.Name
	}
	if combo.Source == "" {
		combo.Source = SourceAdmin
	}
	if combo.Name == "" {
		return Combo{}, fmt.Errorf("combo name is required")
	}
	if len(combo.Name) > maxNameLength {
		return Combo{}, fmt.Errorf("combo name must be %d characters or fewer", maxNameLength)
	}
	if len(combo.Description) > maxDescriptionLength {
		return Combo{}, fmt.Errorf("combo description must be %d characters or fewer", maxDescriptionLength)
	}
	if strings.Contains(combo.Name, "/") {
		return Combo{}, fmt.Errorf("combo name must not contain /")
	}
	if !validComboName.MatchString(combo.Name) {
		return Combo{}, fmt.Errorf("combo name may only contain letters, numbers, dot, dash, and underscore")
	}
	if s.catalog != nil && s.catalog.Supports(combo.Name) {
		return Combo{}, fmt.Errorf("combo name collides with existing model: %s", combo.Name)
	}
	if len(combo.Models) > maxModelsPerCombo {
		return Combo{}, fmt.Errorf("combo supports at most %d models", maxModelsPerCombo)
	}
	for _, model := range combo.Models {
		if len(model) > maxModelSelectorLen {
			return Combo{}, fmt.Errorf("model selector must be %d characters or fewer", maxModelSelectorLen)
		}
	}
	if combo.Enabled && len(combo.Models) < 2 {
		return Combo{}, fmt.Errorf("enabled combo requires at least two models")
	}
	return combo, nil
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
