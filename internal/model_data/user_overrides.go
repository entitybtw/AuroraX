package modeldata

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"aurora/internal/core"
)

// UserOverrides is the YAML schema for data/user_pricing.yaml.
//
// All fields are optional. Pointer types distinguish "set" from "absent",
// allowing per-field merge over the upstream registry without clobbering
// values the user did not touch. Slice and map fields replace the base
// value when set (user fully owns the list / map when present).
type UserOverrides struct {
	// Models overrides entries in ModelList.Models, keyed by model ID
	// (e.g. "gpt-4o", "claude-opus-4").
	Models map[string]ModelOverride `yaml:"models"`

	// ProviderModels overrides entries in ModelList.ProviderModels, keyed
	// by composite "providerType/modelID" (e.g. "openai/gpt-4o").
	ProviderModels map[string]ProviderModelOverride `yaml:"provider_models"`
}

// ModelOverride mirrors the user-editable subset of ModelEntry. Pointer fields
// are merged per-field; slice and map fields replace the base when set.
type ModelOverride struct {
	DisplayName           *string                  `yaml:"display_name,omitempty"`
	Description           *string                  `yaml:"description,omitempty"`
	OwnedBy               *string                  `yaml:"owned_by,omitempty"`
	Family                *string                  `yaml:"family,omitempty"`
	ReleaseDate           *string                  `yaml:"release_date,omitempty"`
	DeprecationDate       *string                  `yaml:"deprecation_date,omitempty"`
	Tags                  []string                 `yaml:"tags,omitempty"`
	Modes                 []string                 `yaml:"modes,omitempty"`
	SourceURL             *string                  `yaml:"source_url,omitempty"`
	Modalities            *Modalities              `yaml:"modalities,omitempty"`
	Capabilities          map[string]bool          `yaml:"capabilities,omitempty"`
	ContextWindow         *int                     `yaml:"context_window,omitempty"`
	MaxOutputTokens       *int                     `yaml:"max_output_tokens,omitempty"`
	MaxImagesPerRequest   *int                     `yaml:"max_images_per_request,omitempty"`
	MaxVideosPerRequest   *int                     `yaml:"max_videos_per_request,omitempty"`
	MaxAudioPerRequest    *int                     `yaml:"max_audio_per_request,omitempty"`
	MaxAudioLengthSeconds *int                     `yaml:"max_audio_length_seconds,omitempty"`
	MaxVideoLengthSeconds *int                     `yaml:"max_video_length_seconds,omitempty"`
	MaxPDFSizeMB          *int                     `yaml:"max_pdf_size_mb,omitempty"`
	OutputVectorSize      *int                     `yaml:"output_vector_size,omitempty"`
	Parameters            map[string]ParameterSpec `yaml:"parameters,omitempty"`
	Rankings              map[string]RankingEntry  `yaml:"rankings,omitempty"`
	Pricing               *core.ModelPricing       `yaml:"pricing,omitempty"`
	Aliases               []string                 `yaml:"aliases,omitempty"`
}

// ProviderModelOverride mirrors the user-editable subset of ProviderModelEntry.
type ProviderModelOverride struct {
	ProviderModelID *string            `yaml:"provider_model_id,omitempty"`
	Enabled         *bool              `yaml:"enabled,omitempty"`
	Pricing         *core.ModelPricing `yaml:"pricing,omitempty"`
	ContextWindow   *int               `yaml:"context_window,omitempty"`
	MaxOutputTokens *int               `yaml:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool    `yaml:"capabilities,omitempty"`
	RateLimits      *RateLimits        `yaml:"rate_limits,omitempty"`
	Endpoints       []string           `yaml:"endpoints,omitempty"`
	Regions         []string           `yaml:"regions,omitempty"`
}

// LoadUserOverrides reads and parses the YAML overrides file at path.
//
// Returns (nil, nil) when path is empty or the file does not exist, so callers
// can treat user overrides as optional. Parse errors are surfaced.
func LoadUserOverrides(path string) (*UserOverrides, error) {
	if path == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading user overrides %q: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var overrides UserOverrides
	if err := yaml.Unmarshal(raw, &overrides); err != nil {
		return nil, fmt.Errorf("parsing user overrides %q: %w", path, err)
	}
	return &overrides, nil
}

// ApplyUserOverrides merges overrides into list in place. Returns counts of
// entries touched. If list or overrides is nil, it is a no-op.
//
// Models / provider_models that exist in overrides but not in the base list
// are inserted as new entries so users can declare metadata for endpoints the
// upstream registry does not yet know about (e.g. local Ollama models). The
// reverse index is rebuilt at the end.
func ApplyUserOverrides(list *ModelList, overrides *UserOverrides) (modelsTouched, providerModelsTouched int) {
	if list == nil || overrides == nil {
		return 0, 0
	}

	if list.Models == nil && len(overrides.Models) > 0 {
		list.Models = make(map[string]ModelEntry, len(overrides.Models))
	}
	for id, override := range overrides.Models {
		base := list.Models[id]
		list.Models[id] = mergeModelEntry(base, override)
		modelsTouched++
	}

	if list.ProviderModels == nil && len(overrides.ProviderModels) > 0 {
		list.ProviderModels = make(map[string]ProviderModelEntry, len(overrides.ProviderModels))
	}
	for key, override := range overrides.ProviderModels {
		base := list.ProviderModels[key]
		list.ProviderModels[key] = mergeProviderModelEntry(base, override)
		providerModelsTouched++
	}

	if modelsTouched > 0 || providerModelsTouched > 0 {
		list.buildReverseIndex()
	}
	return modelsTouched, providerModelsTouched
}

func mergeModelEntry(base ModelEntry, o ModelOverride) ModelEntry {
	if o.DisplayName != nil {
		base.DisplayName = *o.DisplayName
	}
	if o.Description != nil {
		base.Description = o.Description
	}
	if o.OwnedBy != nil {
		base.OwnedBy = o.OwnedBy
	}
	if o.Family != nil {
		base.Family = o.Family
	}
	if o.ReleaseDate != nil {
		base.ReleaseDate = o.ReleaseDate
	}
	if o.DeprecationDate != nil {
		base.DeprecationDate = o.DeprecationDate
	}
	if o.Tags != nil {
		base.Tags = append([]string(nil), o.Tags...)
	}
	if o.Modes != nil {
		base.Modes = append([]string(nil), o.Modes...)
	}
	if o.SourceURL != nil {
		base.SourceURL = o.SourceURL
	}
	if o.Modalities != nil {
		base.Modalities = o.Modalities
	}
	if o.Capabilities != nil {
		base.Capabilities = mergeBoolMap(base.Capabilities, o.Capabilities)
	}
	if o.ContextWindow != nil {
		base.ContextWindow = o.ContextWindow
	}
	if o.MaxOutputTokens != nil {
		base.MaxOutputTokens = o.MaxOutputTokens
	}
	if o.MaxImagesPerRequest != nil {
		base.MaxImagesPerRequest = o.MaxImagesPerRequest
	}
	if o.MaxVideosPerRequest != nil {
		base.MaxVideosPerRequest = o.MaxVideosPerRequest
	}
	if o.MaxAudioPerRequest != nil {
		base.MaxAudioPerRequest = o.MaxAudioPerRequest
	}
	if o.MaxAudioLengthSeconds != nil {
		base.MaxAudioLengthSeconds = o.MaxAudioLengthSeconds
	}
	if o.MaxVideoLengthSeconds != nil {
		base.MaxVideoLengthSeconds = o.MaxVideoLengthSeconds
	}
	if o.MaxPDFSizeMB != nil {
		base.MaxPDFSizeMB = o.MaxPDFSizeMB
	}
	if o.OutputVectorSize != nil {
		base.OutputVectorSize = o.OutputVectorSize
	}
	if o.Parameters != nil {
		base.Parameters = o.Parameters
	}
	if o.Rankings != nil {
		base.Rankings = o.Rankings
	}
	if o.Pricing != nil {
		base.Pricing = mergePricing(base.Pricing, o.Pricing)
	}
	if o.Aliases != nil {
		base.Aliases = append([]string(nil), o.Aliases...)
	}
	return base
}

func mergeProviderModelEntry(base ProviderModelEntry, o ProviderModelOverride) ProviderModelEntry {
	if o.ProviderModelID != nil {
		base.ProviderModelID = o.ProviderModelID
	}
	if o.Enabled != nil {
		base.Enabled = *o.Enabled
	}
	if o.Pricing != nil {
		base.Pricing = mergePricing(base.Pricing, o.Pricing)
	}
	if o.ContextWindow != nil {
		base.ContextWindow = o.ContextWindow
	}
	if o.MaxOutputTokens != nil {
		base.MaxOutputTokens = o.MaxOutputTokens
	}
	if o.Capabilities != nil {
		base.Capabilities = mergeBoolMap(base.Capabilities, o.Capabilities)
	}
	if o.RateLimits != nil {
		base.RateLimits = o.RateLimits
	}
	if o.Endpoints != nil {
		base.Endpoints = append([]string(nil), o.Endpoints...)
	}
	if o.Regions != nil {
		base.Regions = append([]string(nil), o.Regions...)
	}
	return base
}

// mergePricing returns a new ModelPricing where override's non-nil scalar
// fields win, base's other scalars persist. Tiers, when set on override,
// fully replace base.Tiers (they are usually a self-consistent ladder).
func mergePricing(base, override *core.ModelPricing) *core.ModelPricing {
	if override == nil {
		return base
	}
	if base == nil {
		return override.Clone()
	}
	out := base.Clone()
	if override.Currency != "" {
		out.Currency = override.Currency
	}
	if override.InputPerMtok != nil {
		out.InputPerMtok = clonePtr(override.InputPerMtok)
	}
	if override.OutputPerMtok != nil {
		out.OutputPerMtok = clonePtr(override.OutputPerMtok)
	}
	if override.CachedInputPerMtok != nil {
		out.CachedInputPerMtok = clonePtr(override.CachedInputPerMtok)
	}
	if override.CacheWritePerMtok != nil {
		out.CacheWritePerMtok = clonePtr(override.CacheWritePerMtok)
	}
	if override.ReasoningOutputPerMtok != nil {
		out.ReasoningOutputPerMtok = clonePtr(override.ReasoningOutputPerMtok)
	}
	if override.BatchInputPerMtok != nil {
		out.BatchInputPerMtok = clonePtr(override.BatchInputPerMtok)
	}
	if override.BatchOutputPerMtok != nil {
		out.BatchOutputPerMtok = clonePtr(override.BatchOutputPerMtok)
	}
	if override.AudioInputPerMtok != nil {
		out.AudioInputPerMtok = clonePtr(override.AudioInputPerMtok)
	}
	if override.AudioOutputPerMtok != nil {
		out.AudioOutputPerMtok = clonePtr(override.AudioOutputPerMtok)
	}
	if override.PerImage != nil {
		out.PerImage = clonePtr(override.PerImage)
	}
	if override.InputPerImage != nil {
		out.InputPerImage = clonePtr(override.InputPerImage)
	}
	if override.PerSecondInput != nil {
		out.PerSecondInput = clonePtr(override.PerSecondInput)
	}
	if override.PerSecondOutput != nil {
		out.PerSecondOutput = clonePtr(override.PerSecondOutput)
	}
	if override.PerCharacterInput != nil {
		out.PerCharacterInput = clonePtr(override.PerCharacterInput)
	}
	if override.PerRequest != nil {
		out.PerRequest = clonePtr(override.PerRequest)
	}
	if override.PerPage != nil {
		out.PerPage = clonePtr(override.PerPage)
	}
	if len(override.Tiers) > 0 {
		out.Tiers = append([]core.ModelPricingTier(nil), override.Tiers...)
	}
	return out
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func mergeBoolMap(base, override map[string]bool) map[string]bool {
	out := make(map[string]bool, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// SaveUserOverrides writes overrides to path atomically, creating parent directories.
func SaveUserOverrides(path string, overrides *UserOverrides) error {
	if path == "" {
		return fmt.Errorf("user overrides path is empty")
	}
	if overrides == nil {
		overrides = &UserOverrides{}
	}
	raw, err := yaml.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("marshalling user overrides: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("writing temp overrides: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp overrides: %w", err)
	}
	return nil
}

// UpsertProviderModelPricingOverride updates provider_models[selector].pricing.
func UpsertProviderModelPricingOverride(overrides *UserOverrides, selector string, pricing *core.ModelPricing) *UserOverrides {
	out := cloneUserOverrides(overrides)
	if out.ProviderModels == nil {
		out.ProviderModels = make(map[string]ProviderModelOverride)
	}
	o := out.ProviderModels[selector]
	o.Pricing = pricing.Clone()
	out.ProviderModels[selector] = o
	return out
}

// DeleteProviderModelPricingOverride removes only the pricing block for selector.
func DeleteProviderModelPricingOverride(overrides *UserOverrides, selector string) *UserOverrides {
	out := cloneUserOverrides(overrides)
	if out.ProviderModels == nil {
		return out
	}
	o, ok := out.ProviderModels[selector]
	if !ok {
		return out
	}
	o.Pricing = nil
	if providerModelOverrideEmpty(o) {
		delete(out.ProviderModels, selector)
	} else {
		out.ProviderModels[selector] = o
	}
	return out
}

func cloneUserOverrides(in *UserOverrides) *UserOverrides {
	out := &UserOverrides{}
	if in == nil {
		return out
	}
	if len(in.Models) > 0 {
		out.Models = make(map[string]ModelOverride, len(in.Models))
		for k, v := range in.Models {
			if v.Pricing != nil {
				v.Pricing = v.Pricing.Clone()
			}
			out.Models[k] = v
		}
	}
	if len(in.ProviderModels) > 0 {
		out.ProviderModels = make(map[string]ProviderModelOverride, len(in.ProviderModels))
		for k, v := range in.ProviderModels {
			if v.Pricing != nil {
				v.Pricing = v.Pricing.Clone()
			}
			out.ProviderModels[k] = v
		}
	}
	return out
}

func providerModelOverrideEmpty(o ProviderModelOverride) bool {
	return o.ProviderModelID == nil && o.Enabled == nil && o.Pricing == nil &&
		o.ContextWindow == nil && o.MaxOutputTokens == nil && len(o.Capabilities) == 0 &&
		o.RateLimits == nil && len(o.Endpoints) == 0 && len(o.Regions) == 0
}
