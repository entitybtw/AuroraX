package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readPublished proxies the manager published for the sidecar to rotate over.
func readPublished(t *testing.T, file string) []struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
} {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read published proxies: %v", err)
	}
	var payload struct {
		Proxies []struct {
			IP   string `json:"ip"`
			Port int    `json:"port"`
		} `json:"proxies"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("published proxies json: %v (%s)", err, raw)
	}
	return payload.Proxies
}

func TestSidecarBindProxies_FollowsTheProviderAddresses(t *testing.T) {
	t.Setenv("AURORA_SIDECAR_ENABLED", "true")
	t.Setenv("AURORA_SIDECAR_BIND_PROXIES", "")
	file := filepath.Join(t.TempDir(), "sidecar-bind-proxies.json")
	manager := newSidecarBindProxies(file)
	t.Cleanup(manager.Stop)

	// Two addresses bound to providers: both get a local CONNECT proxy.
	manager.Sync([]string{"127.0.0.1", "127.0.0.2"})
	first := readPublished(t, file)
	if len(first) != 2 {
		t.Fatalf("published %d proxies, want 2 (one per address): %+v", len(first), first)
	}
	owned := len(manager.owned)
	if owned != 2 {
		t.Fatalf("gateway owns %d listeners, want 2", owned)
	}

	// The same set must not churn: same ports, no rebind.
	manager.Sync([]string{"127.0.0.1", "127.0.0.2"})
	second := readPublished(t, file)
	if len(second) != 2 || second[0].Port != first[0].Port || second[1].Port != first[1].Port {
		t.Fatalf("unchanged set rebound: %+v -> %+v", first, second)
	}

	// Dropping an address closes its listener and unlists it: that is how an
	// address removed from every provider leaves the rotation.
	manager.Sync([]string{"127.0.0.1"})
	third := readPublished(t, file)
	if len(third) != 1 {
		t.Fatalf("published %d proxies after removal, want 1: %+v", len(third), third)
	}
	if len(manager.owned) != 1 {
		t.Fatalf("gateway still owns %d listeners, want 1", len(manager.owned))
	}
	removed := second[1]
	if removed.IP == third[0].IP {
		removed = second[0]
	}
	if conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", removed.Port)); err == nil {
		_ = conn.Close()
		t.Fatalf("listener for %s still accepting after removal", removed.IP)
	}
}

func TestSidecarBindProxies_AdoptsEntrypointProxies(t *testing.T) {
	t.Setenv("AURORA_SIDECAR_ENABLED", "true")
	// The entrypoint already listens for this address: adopting it must not
	// bind a second listener.
	t.Setenv("AURORA_SIDECAR_BIND_PROXIES", "127.0.0.1:8981")
	file := filepath.Join(t.TempDir(), "sidecar-bind-proxies.json")
	manager := newSidecarBindProxies(file)
	t.Cleanup(manager.Stop)

	manager.Sync([]string{"127.0.0.1"})
	published := readPublished(t, file)
	if len(published) != 1 || published[0].Port != 8981 {
		t.Fatalf("published %+v, want the entrypoint's 127.0.0.1:8981", published)
	}
	if len(manager.owned) != 0 {
		t.Fatalf("gateway bound its own listener for an address the entrypoint owns: %+v", manager.owned)
	}

	// An address with no entrypoint listener is still added alongside it.
	manager.Sync([]string{"127.0.0.1", "127.0.0.2"})
	published = readPublished(t, file)
	if len(published) != 2 {
		t.Fatalf("published %+v, want the adopted and the new address", published)
	}
	if len(manager.owned) != 1 {
		t.Fatalf("gateway owns %d listeners, want 1 (the new address)", len(manager.owned))
	}
}

func TestSidecarBindProxies_DisabledWhenTheSidecarIsOff(t *testing.T) {
	t.Setenv("AURORA_SIDECAR_ENABLED", "false")
	t.Setenv("AURORA_SIDECAR_BIND_PROXIES", "")
	file := filepath.Join(t.TempDir(), "sidecar-bind-proxies.json")
	manager := newSidecarBindProxies(file)
	t.Cleanup(manager.Stop)

	manager.Sync([]string{"127.0.0.1"})
	if published := readPublished(t, file); len(published) != 0 {
		t.Fatalf("published %+v while the sidecar is disabled", published)
	}
	if len(manager.owned) != 0 {
		t.Fatalf("gateway owns listeners while the sidecar is disabled")
	}
}

// The proxy has to relay a real CONNECT tunnel — the whole point of the
// listener — with the configured address as the source of the outbound dial.
func TestSidecarBindProxies_ConnectTunnelWorks(t *testing.T) {
	t.Setenv("AURORA_SIDECAR_ENABLED", "true")
	t.Setenv("AURORA_SIDECAR_BIND_PROXIES", "")
	file := filepath.Join(t.TempDir(), "sidecar-bind-proxies.json")
	manager := newSidecarBindProxies(file)
	t.Cleanup(manager.Stop)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "through the tunnel")
	}))
	defer upstream.Close()

	manager.Sync([]string{"127.0.0.1"})
	published := readPublished(t, file)
	if len(published) != 1 {
		t.Fatalf("published %+v, want one proxy", published)
	}
	target := strings.TrimPrefix(upstream.URL, "http://")
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", published[0].Port))
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		t.Fatalf("CONNECT: %v", err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d, want 200", resp.StatusCode)
	}

	if _, err := fmt.Fprintf(conn, "GET /hello HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target); err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, err := io.ReadAll(br)
	if err != nil {
		t.Fatalf("read tunnelled response: %v", err)
	}
	if !strings.Contains(string(body), "through the tunnel") {
		t.Fatalf("tunnelled response = %q", body)
	}
}

func TestNormaliseIPs_CanonicalisesAndSorts(t *testing.T) {
	got := normaliseIPs([]string{" 2001:0db8::1 ", "10.0.0.2", "10.0.0.2", "not-an-ip", "", "10.0.0.1"})
	want := []string{"10.0.0.1", "10.0.0.2", "2001:db8::1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("normaliseIPs = %v, want %v", got, want)
	}
}
