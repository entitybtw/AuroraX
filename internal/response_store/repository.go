package responsestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurora/internal/core"
)

var ErrNotFound = errors.New("response not found")

type StoredResponse struct {
	Response           *core.ResponsesResponse `json:"response"`
	InputItems         []json.RawMessage       `json:"input_items,omitempty"`
	Provider           string                  `json:"provider,omitempty"`
	ProviderName       string                  `json:"provider_name,omitempty"`
	ProviderResponseID string                  `json:"provider_response_id,omitempty"`
	RequestID          string                  `json:"request_id,omitempty"`
	UserPath           string                  `json:"user_path,omitempty"`
	WorkflowVersionID  string                  `json:"workflow_version_id,omitempty"`
	StoredAt           time.Time               `json:"stored_at,omitempty"`
	ExpiresAt          time.Time               `json:"expires_at,omitempty"`
}

type Store interface {
	Create(ctx context.Context, response *StoredResponse) error
	Get(ctx context.Context, id string) (*StoredResponse, error)
	Update(ctx context.Context, response *StoredResponse) error
	Delete(ctx context.Context, id string) error
	Close() error
}

func copyResponse(src *StoredResponse) (*StoredResponse, error) {
	if src == nil {
		return nil, fmt.Errorf("source response is nil")
	}
	normalized := normalizeResponse(src)
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	var dst StoredResponse
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &dst, nil
}

func normalizeResponse(src *StoredResponse) *StoredResponse {
	if src == nil {
		return nil
	}
	out := *src
	out.Provider = strings.TrimSpace(out.Provider)
	out.ProviderName = strings.TrimSpace(out.ProviderName)
	out.ProviderResponseID = strings.TrimSpace(out.ProviderResponseID)
	out.RequestID = strings.TrimSpace(out.RequestID)
	out.UserPath = strings.TrimSpace(out.UserPath)
	out.WorkflowVersionID = strings.TrimSpace(out.WorkflowVersionID)
	if src.Response != nil {
		rc := *src.Response
		if rc.Provider == "" {
			rc.Provider = out.Provider
		}
		if out.Provider == "" {
			out.Provider = strings.TrimSpace(rc.Provider)
		}
		if out.ProviderResponseID == "" {
			out.ProviderResponseID = strings.TrimSpace(rc.ID)
		}
		out.Response = &rc
	}
	if len(src.InputItems) > 0 {
		out.InputItems = make([]json.RawMessage, 0, len(src.InputItems))
		for _, item := range src.InputItems {
			out.InputItems = append(out.InputItems, core.CloneRawJSON(item))
		}
	}
	return &out
}
