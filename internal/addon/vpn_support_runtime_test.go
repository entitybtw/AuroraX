package addon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The shipped vpn-support runtime is interpreted by Yaegi, which trails the
// installed Go release. Yaegi miscompiles `len()` written directly inside a
// multi-value return: the destination slot is typed for the whole tuple, so
// SetInt lands on a slice and the addon panics on its refresh goroutine,
// taking the gateway down with it. The runtime computes the length into a
// named variable first; this test replays the whole refresh path against the
// real source so a regression is caught before the extension is installed.
//
// The store source lives outside this repository, so the test is opt-in:
//
//	VPN_RUNTIME=../aurorax-store/extensions/vpn-support/vpn-runtime.go \
//	    go test ./internal/addon/ -run VPNSupport
func VPNSupportRuntimeSource(t *testing.T) string {
	t.Helper()
	path := os.Getenv("VPN_RUNTIME")
	if path == "" {
		t.Skip("VPN_RUNTIME not set")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(src)
}

func vpnsPayload(dir, provider, key string, cfg map[string]string) string {
	raw, err := json.Marshal(map[string]any{
		"hook":      "test",
		"extension": "vpn-support",
		"provider":  provider,
		"dir":       dir,
		"key":       key,
		"settings":  map[string]string{},
		"config":    cfg,
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// vpnsState is the persisted runtime state the addon writes after a refresh.
type vpnsState struct {
	LastRefresh   time.Time `json:"last_refresh"`
	Selected      []string  `json:"selected"`
	Subscriptions int       `json:"subscriptions"`
	SettingsFP    string    `json:"settings_fp"`
}

func readVpnState(t *testing.T, dir string) vpnsState {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "vpn-support-state.json"))
	if err != nil {
		t.Fatalf("state file: %v", err)
	}
	var state vpnsState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state json: %v (%s)", err, raw)
	}
	return state
}

func TestVPNSupportRuntimeRefreshUnderYaegi(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vpn-runtime.go"), []byte(VPNSupportRuntimeSource(t)), 0o600); err != nil {
		t.Fatal(err)
	}

	// Local listener so TCP probes succeed and endpoints stay alive.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	vmessJSON := fmt.Sprintf(`{"add":"127.0.0.1","port":%d,"ps":"DE Frankfurt 03","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"tcp","type":"none"}`, port)
	plain := strings.Join([]string{
		fmt.Sprintf("vless://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee@127.0.0.1:%d?type=tcp&security=none#US Ashburn 01", port),
		fmt.Sprintf("trojan://secret@127.0.0.1:%d#NL Amsterdam 02", port),
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessJSON)),
		"this-is-not-a-uri",
	}, "\n")
	b64 := base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("vless://aaaaaaaa-bbbb-cccc-dddd-ffffffffffff@127.0.0.1:%d?type=tcp#JP Tokyo 05", port),
	))

	srvPlain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(plain))
	}))
	defer srvPlain.Close()
	srvB64 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(b64))
	}))
	defer srvB64.Close()

	store := NewStore(dir)
	store.Load()
	a := store.Get("vpn-runtime")
	if a == nil {
		t.Fatalf("addon did not load: %v", store.List())
	}

	cfg := map[string]string{
		"apply_to":                 "vllm-zen,vllm-cheapvibecode",
		"subscriptions":            srvPlain.URL + "\n" + srvB64.URL + "\nvless://aaaaaaaa-bbbb-cccc-dddd-999999999999@127.0.0.1:" + fmt.Sprint(port) + "#Inline 09",
		"refresh_interval_minutes": "1",
		"node_count":               "2",
		"node_order":               "best_first",
		"protocols":                "vless,vmess,ss,trojan",
		"quality_max_ms":           "0",
		"probe_count":              "2",
		"probe_timeout_ms":         "400",
		"egress_mode":              "rotate",
		"core_kind":                "static",
		"local_socks_ports":        "1080,1081",
	}

	if out, err := a.CallString("OnInit", vpnsPayload(dir, "vllm-zen", "", cfg)); err != nil {
		t.Fatalf("OnInit: %v", err)
	} else if !strings.Contains(out, `"ok":true`) {
		t.Errorf("OnInit = %s", out)
	}

	// A provider outside apply_to must not receive exits.
	other := map[string]string{}
	for k, v := range cfg {
		other[k] = v
	}
	other["apply_to"] = "openrouter-main"
	if out, err := a.CallString("EgressCandidates", vpnsPayload(dir, "openrouter-main", "", other)); err != nil {
		t.Fatalf("EgressCandidates (out of scope): %v", err)
	} else if strings.Contains(out, `"name"`) {
		t.Errorf("out-of-scope provider got candidates: %s", out)
	}

	// A matching provider kicks off the background refresh: this is the call
	// that panicked before len() was hoisted out of the return statement.
	if out, err := a.CallString("EgressCandidates", vpnsPayload(dir, "vllm-zen", "", cfg)); err != nil {
		t.Fatalf("EgressCandidates: %v", err)
	} else if !strings.Contains(out, `"candidates":[]`) {
		t.Errorf("expected no candidates before the first refresh: %s", out)
	}
	time.Sleep(8 * time.Second)

	data, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "", cfg))
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if !strings.Contains(data, `"k":"Endpoints","v":"4"`) {
		t.Errorf("status does not report 4 endpoints: %s", data)
	}

	nodes, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "nodes", cfg))
	if err != nil {
		t.Fatalf("Data(nodes): %v", err)
	}
	for _, want := range []string{"US Ashburn 01", "DE Frankfurt 03", "JP Tokyo 05", "Inline 09"} {
		if !strings.Contains(nodes, want) {
			t.Errorf("IP list is missing %q: %s", want, nodes)
		}
	}

	// The dashboard reads a machine-readable endpoint list next to pools and
	// providers: the same endpoints plus the flags it renders, and nothing of
	// the prose the "nodes" key is formatted for humans.
	servers, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "servers", cfg))
	if err != nil {
		t.Fatalf("Data(servers): %v", err)
	}
	for _, want := range []string{
		`"servers":[`,
		`"apply_to":"vllm-zen,vllm-cheapvibecode"`,
		`"egress_mode":"rotate"`,
		`"core_kind":"static"`,
		`"host":`,
		`"selected":`,
	} {
		if !strings.Contains(servers, want) {
			t.Errorf("endpoint list is missing %q: %s", want, servers)
		}
	}
	if strings.Contains(servers, `"blocks"`) {
		t.Errorf("endpoint list should not carry UI blocks: %s", servers)
	}

	candidates, err := a.CallString("EgressCandidates", vpnsPayload(dir, "vllm-zen", "", cfg))
	if err != nil {
		t.Fatalf("EgressCandidates (after): %v", err)
	}
	if !strings.Contains(candidates, "socks5://127.0.0.1:1080") ||
		!strings.Contains(candidates, "socks5://127.0.0.1:1081") {
		t.Errorf("vpn exits not emitted: %s", candidates)
	}
	if !strings.Contains(candidates, `"tier":0`) {
		t.Errorf("rotate mode should stay on tier 0: %s", candidates)
	}

	if out, err := a.CallString("OnSettingsSave", vpnsPayload(dir, "vllm-zen", "", cfg)); err != nil {
		t.Fatalf("OnSettingsSave: %v", err)
	} else if !strings.Contains(out, `"ok":true`) {
		t.Errorf("OnSettingsSave = %s", out)
	}
	time.Sleep(6 * time.Second)

	first := readVpnState(t, dir)
	if len(first.Selected) != 2 {
		t.Errorf("selected = %v, want 2 kept IPs", first.Selected)
	}
	if first.Subscriptions != 3 {
		t.Errorf("subscriptions = %d, want 3", first.Subscriptions)
	}
	if first.SettingsFP == "" {
		t.Error("state has no settings fingerprint")
	}

	// The gateway never invokes OnSettingsSave, so an edited setting has to
	// invalidate the last refresh on its own. Without that the operator waits
	// out the whole interval after pasting a subscription.
	edited := map[string]string{}
	for k, v := range cfg {
		edited[k] = v
	}
	edited["node_count"] = "1"
	if _, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "", edited)); err != nil {
		t.Fatalf("Data (edited): %v", err)
	}
	time.Sleep(6 * time.Second)

	second := readVpnState(t, dir)
	if second.SettingsFP == first.SettingsFP {
		t.Error("settings fingerprint did not change after editing node_count")
	}
	if !second.LastRefresh.After(first.LastRefresh) {
		t.Errorf("edited setting did not trigger a refresh: %v -> %v", first.LastRefresh, second.LastRefresh)
	}
	if len(second.Selected) != 1 {
		t.Errorf("selected = %v, want 1 kept IP after node_count=1", second.Selected)
	}

	// Unchanged settings must not spin the fetcher on every dashboard poll.
	if _, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "", edited)); err != nil {
		t.Fatalf("Data (repeat): %v", err)
	}
	if _, err := a.CallString("EgressCandidates", vpnsPayload(dir, "vllm-zen", "", edited)); err != nil {
		t.Fatalf("EgressCandidates (repeat): %v", err)
	}
	time.Sleep(3 * time.Second)
	third := readVpnState(t, dir)
	if !third.LastRefresh.Equal(second.LastRefresh) {
		t.Errorf("refresh loop with unchanged settings: %v -> %v", second.LastRefresh, third.LastRefresh)
	}
}
