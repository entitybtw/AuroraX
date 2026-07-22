package batch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aurora/internal/core"
)

var ErrNotFound = errors.New("batch not found")

const (
	RequestIDMetadataKey     = "request_id"
	UsageLoggedAtMetadataKey = "usage_logged_at"
)

type StoredBatch struct {
	Batch                     *core.BatchResponse `json:"batch"`
	RequestEndpointByCustomID map[string]string   `json:"request_endpoint_by_custom_id,omitempty"`
	OriginalInputFileID       string              `json:"original_input_file_id,omitempty"`
	RewrittenInputFileID      string              `json:"rewritten_input_file_id,omitempty"`
	RequestID                 string              `json:"request_id,omitempty"`
	UserPath                  string              `json:"user_path,omitempty"`
	WorkflowVersionID         string              `json:"workflow_version_id,omitempty"`
	UsageEnabled              *bool               `json:"usage_enabled,omitempty"`
	UsageLoggedAt             *time.Time          `json:"usage_logged_at,omitempty"`
}

type Store interface {
	Create(ctx context.Context, batch *StoredBatch) error
	Get(ctx context.Context, id string) (*StoredBatch, error)
	List(ctx context.Context, limit int, after string) ([]*StoredBatch, error)
	Update(ctx context.Context, batch *StoredBatch) error
	Close() error
}

func clampPageSize(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 101:
		return 101
	default:
		return limit
	}
}

func deepCopy(src *StoredBatch) (*StoredBatch, error) {
	if src == nil {
		return nil, fmt.Errorf("source batch is nil")
	}
	n := normalizeBatch(src)
	data, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}
	var dst StoredBatch
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, fmt.Errorf("unmarshal batch: %w", err)
	}
	return &dst, nil
}

func encodeBatch(batch *StoredBatch) ([]byte, error) {
	if batch == nil {
		return nil, fmt.Errorf("batch is nil")
	}
	n := normalizeBatch(batch)
	if n.Batch == nil {
		return nil, fmt.Errorf("batch payload is nil")
	}
	if len(n.Batch.ID) == 0 {
		return nil, fmt.Errorf("batch ID is empty")
	}
	data, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}
	return data, nil
}

func decodeBatch(raw []byte) (*StoredBatch, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty batch payload")
	}
	var stored StoredBatch
	if err := json.Unmarshal(raw, &stored); err == nil && stored.Batch != nil && stored.Batch.ID != "" {
		return normalizeBatch(&stored), nil
	}
	var legacy core.BatchResponse
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshal batch: %w", err)
	}
	if legacy.ID == "" {
		return nil, fmt.Errorf("legacy batch missing ID")
	}
	return normalizeBatch(&StoredBatch{Batch: &legacy}), nil
}

func normalizeBatch(src *StoredBatch) *StoredBatch {
	if src == nil {
		return nil
	}
	out := *src
	if src.Batch == nil {
		return &out
	}
	copy := *src.Batch
	copy.Metadata, out.RequestID, out.UsageLoggedAt = extractGatewayKeys(
		src.Batch.Metadata,
		out.RequestID,
		out.UsageLoggedAt,
	)
	out.Batch = &copy
	return &out
}

func extractGatewayKeys(metadata map[string]string, requestID string, usageLoggedAt *time.Time) (map[string]string, string, *time.Time) {
	if len(metadata) == 0 {
		return nil, strings.TrimSpace(requestID), usageLoggedAt
	}
	public := make(map[string]string, len(metadata))
	rid := strings.TrimSpace(requestID)
	ula := usageLoggedAt
	for k, v := range metadata {
		switch k {
		case RequestIDMetadataKey:
			if rid == "" {
				rid = strings.TrimSpace(v)
			}
		case UsageLoggedAtMetadataKey:
			if ula == nil {
				ula = parseLoggedAt(v)
			}
		default:
			public[k] = v
		}
	}
	if len(public) == 0 {
		public = nil
	}
	return public, rid, ula
}

func parseLoggedAt(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if unixSec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		ts := time.Unix(unixSec, 0).UTC()
		return &ts
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		ts := parsed.UTC()
		return &ts
	}
	return nil
}

func (s *StoredBatch) EffectiveUsageEnabled() bool {
	return s == nil || s.UsageEnabled == nil || *s.UsageEnabled
}
