package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/gateway"
)

func TestApplyResponseHeaders(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.ResponseHeadersConfig
		meta       gateway.ExecutionMeta
		requested  string
		wantActual map[string]string
		wantUnset  []string
	}{
		{
			name: "disabled emits nothing",
			cfg:  config.ResponseHeadersConfig{Enabled: false, IncludeFallback: true, IncludeNonFallback: true},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o"},
			wantActual: map[string]string{},
		},
		{
			name: "non-fallback with include_non_fallback",
			cfg: config.ResponseHeadersConfig{Enabled: true, IncludeFallback: false, IncludeNonFallback: true,
				ActualProviderHeader: true, ActualModelHeader: true, RequestedModelHeader: true, FallbackChainHeader: true},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "primary",
				"X-Actual-Model":    "gpt-4o",
				"X-Requested-Model": "gpt-4o",
			},
		},
		{
			name: "non-fallback suppressed when include_non_fallback disabled",
			cfg:  config.ResponseHeadersConfig{Enabled: true, IncludeFallback: true, IncludeNonFallback: false},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			wantActual: map[string]string{},
		},
		{
			name: "fallback emits chain and failover model",
			cfg: config.ResponseHeadersConfig{Enabled: true, IncludeFallback: true, IncludeNonFallback: false,
				ActualProviderHeader: true, ActualModelHeader: true, RequestedModelHeader: true, FallbackChainHeader: true},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "fallback-inst", Model: "gpt-4o-mini", UsedFallback: true, FallbackChain: []string{"gpt-4o", "gpt-4o-mini"}},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "fallback-inst",
				"X-Actual-Model":    "gpt-4o-mini",
				"X-Requested-Model": "gpt-4o",
				"X-Fallback-Chain":  "gpt-4o,gpt-4o-mini",
			},
		},
		{
			name: "fallback suppressed when include_fallback disabled",
			cfg:  config.ResponseHeadersConfig{Enabled: true, IncludeFallback: false, IncludeNonFallback: true},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "fb", Model: "m", UsedFallback: true, FallbackChain: []string{"a", "b"}},
			wantActual: map[string]string{},
		},
		{
			name: "provider falls back to type when name empty",
			cfg: config.ResponseHeadersConfig{Enabled: true, IncludeFallback: false, IncludeNonFallback: true,
				ActualProviderHeader: true, ActualModelHeader: true, RequestedModelHeader: true, FallbackChainHeader: true},
			meta: gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "openai",
				"X-Actual-Model":    "gpt-4o",
				"X-Requested-Model": "gpt-4o",
			},
		},
		{
			name: "individual header flags disable specific headers",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      false,
				IncludeNonFallback:   true,
				ActualProviderHeader: true,
				ActualModelHeader:    false,
				RequestedModelHeader: true,
				FallbackChainHeader:  false,
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false, FallbackChain: []string{"a", "b"}},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "primary",
				"X-Requested-Model": "gpt-4o",
			},
			wantUnset: []string{"X-Actual-Model", "X-Fallback-Chain"},
		},
		{
			name: "custom header emitted with placeholder expansion",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      false,
				IncludeNonFallback:   true,
				ActualProviderHeader: true,
				ActualModelHeader:    true,
				RequestedModelHeader: true,
				FallbackChainHeader:  true,
				CustomHeaders: []config.CustomResponseHeaderConfig{
					{Name: "X-Custom", Value: "{actual_model} via {actual_provider}", Enabled: true},
				},
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Custom": "gpt-4o via primary",
			},
		},
		{
			name: "disabled custom header is skipped",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      false,
				IncludeNonFallback:   true,
				ActualProviderHeader: true,
				ActualModelHeader:    true,
				RequestedModelHeader: true,
				FallbackChainHeader:  true,
				CustomHeaders: []config.CustomResponseHeaderConfig{
					{Name: "X-Skip", Value: "value", Enabled: false},
				},
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "primary",
			},
			wantUnset: []string{"X-Skip"},
		},
		{
			name: "custom header with empty name is skipped",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      false,
				IncludeNonFallback:   true,
				ActualProviderHeader: true,
				ActualModelHeader:    true,
				RequestedModelHeader: true,
				FallbackChainHeader:  true,
				CustomHeaders: []config.CustomResponseHeaderConfig{
					{Name: "   ", Value: "value", Enabled: true},
				},
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "primary",
			},
		},
		{
			name: "custom header with empty value is skipped",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      false,
				IncludeNonFallback:   true,
				ActualProviderHeader: true,
				ActualModelHeader:    true,
				RequestedModelHeader: true,
				FallbackChainHeader:  true,
				CustomHeaders: []config.CustomResponseHeaderConfig{
					{Name: "X-Empty", Value: "   ", Enabled: true},
				},
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{
				"X-Actual-Provider": "primary",
			},
			wantUnset: []string{"X-Empty"},
		},
		{
			name: "custom headers suppressed when include_non_fallback disabled",
			cfg: config.ResponseHeadersConfig{
				Enabled:              true,
				IncludeFallback:      true,
				IncludeNonFallback:   false,
				ActualProviderHeader: true,
				ActualModelHeader:    true,
				RequestedModelHeader: true,
				FallbackChainHeader:  true,
				CustomHeaders: []config.CustomResponseHeaderConfig{
					{Name: "X-Custom", Value: "value", Enabled: true},
				},
			},
			meta:      gateway.ExecutionMeta{ProviderType: "openai", ProviderName: "primary", Model: "gpt-4o", UsedFallback: false},
			requested: "gpt-4o",
			wantActual: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			svc := &translatedInferenceService{}
			svc.setResponseHeadersConfig(tt.cfg)

			workflow := &core.Workflow{
				Resolution: &core.RequestModelResolution{
					Requested: core.NewRequestedModelSelector(tt.requested, ""),
				},
			}

			svc.applyResponseHeaders(c, workflow, tt.meta)

			headers := rec.Header()
			for key, want := range tt.wantActual {
				if got := headers.Get(key); got != want {
					t.Errorf("header %s = %q, want %q", key, got, want)
				}
			}
			if len(tt.wantActual) == 0 {
				for _, key := range []string{"X-Actual-Provider", "X-Actual-Model", "X-Requested-Model", "X-Fallback-Chain", "X-Custom", "X-Skip", "X-Empty"} {
					if got := headers.Get(key); got != "" {
						t.Errorf("header %s = %q, want unset", key, got)
					}
				}
			}
			for _, key := range tt.wantUnset {
				if got := headers.Get(key); got != "" {
					t.Errorf("header %s = %q, want unset", key, got)
				}
			}
		})
	}
}