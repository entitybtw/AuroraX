package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var (
	TotalRequests int64
	StartTime     time.Time
)

// InitHeartbeat starts the background heartbeat loop
func InitHeartbeat(ctx context.Context, version string) {
	if os.Getenv("AURORA_TELEMETRY_DISABLE") == "true" {
		slog.Info("telemetry heartbeat disabled")
		return
	}

	StartTime = time.Now()
	go runHeartbeatLoop(ctx, version)
}

// IncrementRequestCount increments the total count of model interaction requests
func IncrementRequestCount() {
	atomic.AddInt64(&TotalRequests, 1)
}

func runHeartbeatLoop(ctx context.Context, version string) {
	id := getOrCreateInstanceID()
	if id == "" {
		slog.Debug("could not generate or read instance id, telemetry disabled")
		return
	}

	// Wait 2 minutes after startup before sending the first heartbeat
	// to avoid spamming on rapid restarts or local dev reloading.
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Minute):
	}

	sendHeartbeat(id, version)

	// Send every 2 hours
	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendHeartbeat(id, version)
		}
	}
}

func getOrCreateInstanceID() string {
	paths := []string{}

	// 1. Home directory
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".aurora")
		_ = os.MkdirAll(dir, 0755)
		paths = append(paths, filepath.Join(dir, "instance.id"))
	}

	// 2. Data directory (common container mount)
	_ = os.MkdirAll("data", 0755)
	paths = append(paths, filepath.Join("data", "instance.id"))

	// 3. Current working directory
	paths = append(paths, "instance.id")

	// Read existing from any path
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			id := string(bytes.TrimSpace(data))
			if _, err := uuid.Parse(id); err == nil {
				// Re-sync this ID to other writeable paths if they are missing it
				for _, otherPath := range paths {
					if otherPath != p {
						_ = os.WriteFile(otherPath, []byte(id), 0644)
					}
				}
				return id
			}
		}
	}

	// Generate new
	id := uuid.New().String()
	for _, p := range paths {
		_ = os.WriteFile(p, []byte(id), 0644)
	}
	return id
}

func sendHeartbeat(id, version string) {
	uptime := time.Since(StartTime)
	totalReqs := atomic.LoadInt64(&TotalRequests)
	
	// Calculate average Requests Per Second (RPS)
	rps := float64(totalReqs) / uptime.Seconds()

	bucket := "<100"
	switch {
	case rps >= 10000:
		bucket = "10k+"
	case rps >= 1000:
		bucket = "1k-10k"
	case rps >= 100:
		bucket = "100-1k"
	}

	payload := map[string]any{
		"instance_id":    id,
		"version":        version,
		"rps_bucket":     bucket,
		"uptime_seconds": int(uptime.Seconds()),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := os.Getenv("AURORA_TELEMETRY_URL")
	if url == "" {
		url = "https://aurora-heartbeat.cortexx.workers.dev/api/v1/heartbeat"
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("telemetry heartbeat post failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Debug("telemetry heartbeat sent successfully")
	} else {
		slog.Debug("telemetry heartbeat returned non-200 status", "status", resp.StatusCode)
	}
}
