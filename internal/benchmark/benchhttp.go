package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurora/internal/core"
)

// NewBenchHTTPHandler creates an http.Handler that routes to the mock provider.
// This is a minimal handler that bypasses the full gateway server stack,
// useful for in-process benchmarks without importing the server package.
func NewBenchHTTPHandler(mock *MockProvider) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req core.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp, err := mock.ChatCompletion(context.Background(), &req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		var req core.ResponsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp, err := mock.Responses(context.Background(), &req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		models, _ := mock.ListModels(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{\"status\":\"ok\"}`))
	})
	return mux
}

// DefaultLoadTestConfig returns sensible defaults for an in-process benchmark.
func DefaultLoadTestConfig(handler http.Handler) LoadTestConfig {
	return LoadTestConfig{
		Concurrency:    50,
		Duration:       10 * time.Second,
		WarmupDuration: 2 * time.Second,
		RampUpDuration: 2 * time.Second,
		Endpoint:       "/v1/chat/completions",
		Method:         "POST",
		RequestBody:    []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}`),
		Mode:           ModeInProcess,
		Server:         handler,
	}
}
