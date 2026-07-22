package responsecache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"aurora/configuration"
)

func TestQdrantSearchEnsuresPayloadIndexes(t *testing.T) {
	var indexFields []string
	var searched bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/aurora_semantic":
			writeJSON(t, w, map[string]any{
				"result": map[string]any{
					"config": map[string]any{
						"params": map[string]any{
							"vectors": map[string]any{"size": 3, "distance": "Cosine"},
						},
					},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/aurora_semantic/index":
			var body struct {
				FieldName   string `json:"field_name"`
				FieldSchema string `json:"field_schema"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode index body: %v", err)
			}
			indexFields = append(indexFields, body.FieldName+":"+body.FieldSchema)
			writeJSON(t, w, map[string]any{"result": map[string]any{"status": "acknowledged"}})
		case r.Method == http.MethodPost && r.URL.Path == "/collections/aurora_semantic/points/search":
			searched = true
			writeJSON(t, w, map[string]any{"result": []any{}})
		default:
			t.Fatalf("unexpected qdrant request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	store, err := newQdrantStore(config.QdrantConfig{URL: srv.URL, Collection: "aurora_semantic"})
	if err != nil {
		t.Fatalf("newQdrantStore: %v", err)
	}
	defer store.Close()

	_, err = store.Search(context.Background(), []float32{1, 0, 0}, "params", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	want := []string{"params_hash:keyword", "expires_at:integer"}
	if !reflect.DeepEqual(indexFields, want) {
		t.Fatalf("index fields = %#v, want %#v", indexFields, want)
	}
	if !searched {
		t.Fatal("expected search request after readiness checks")
	}
}

func TestQdrantInsertCreatesCollectionAndPayloadIndexes(t *testing.T) {
	var createdCollection bool
	var indexFields []string
	var upserted bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/aurora_semantic":
			http.NotFound(w, r)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/aurora_semantic":
			createdCollection = true
			writeJSON(t, w, map[string]any{"result": true})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/aurora_semantic/index":
			var body struct {
				FieldName   string `json:"field_name"`
				FieldSchema string `json:"field_schema"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode index body: %v", err)
			}
			indexFields = append(indexFields, body.FieldName+":"+body.FieldSchema)
			writeJSON(t, w, map[string]any{"result": map[string]any{"status": "acknowledged"}})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/aurora_semantic/points" && r.URL.Query().Get("wait") == "true":
			upserted = true
			writeJSON(t, w, map[string]any{"result": map[string]any{"status": "completed"}})
		default:
			t.Fatalf("unexpected qdrant request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer srv.Close()

	store, err := newQdrantStore(config.QdrantConfig{URL: srv.URL, Collection: "aurora_semantic"})
	if err != nil {
		t.Fatalf("newQdrantStore: %v", err)
	}
	defer store.Close()

	err = store.Insert(context.Background(), "cache-key", []float32{1, 0, 0}, []byte(`{"ok":true}`), "params", "What is DFS?", 0)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if !createdCollection {
		t.Fatal("expected collection creation")
	}
	want := []string{"params_hash:keyword", "expires_at:integer"}
	if !reflect.DeepEqual(indexFields, want) {
		t.Fatalf("index fields = %#v, want %#v", indexFields, want)
	}
	if !upserted {
		t.Fatal("expected upsert after collection and index readiness")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
