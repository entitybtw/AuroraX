package httpclient

import (
	"io"
	"net/http"
	"testing"
	"time"
)

func TestUTLSupstreamModels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	client := NewUTLSHTTPClient()
	client.Timeout = 30 * time.Second

	req, err := http.NewRequest("GET", "https://opencode.ai/zen/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "opencode/1.0")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	t.Logf("status=%d proto=%s len=%d", resp.StatusCode, resp.Proto, len(body))
	if len(body) > 0 {
		n := len(body)
		if n > 500 {
			n = 500
		}
		t.Logf("body[:%d]=%s", n, string(body[:n]))
	}

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}
