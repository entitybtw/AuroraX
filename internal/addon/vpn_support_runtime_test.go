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
	"strconv"
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
	SelectedKeys  []string  `json:"selected_keys"`
	Subscriptions int       `json:"subscriptions"`
	SettingsFP    string    `json:"settings_fp"`
}

// vpnsCandidate is one exit the addon offers to the gateway.
type vpnsCandidate struct {
	Name  string `json:"name"`
	Proxy string `json:"proxy"`
	Tier  int    `json:"tier"`
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
	acceptLoop := func(l net.Listener) {
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				_ = conn.Close()
			}
		}()
	}
	acceptLoop(ln)
	port := ln.Addr().(*net.TCPAddr).Port

	// A second address carrying the same display label as the US entry: a
	// subscription repeats labels freely, and the addon must keep the two
	// nodes apart instead of burning a slot on the label.
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln2.Close() }()
	acceptLoop(ln2)
	port2 := ln2.Addr().(*net.TCPAddr).Port

	// One more listener per remaining node: two nodes sharing protocol and
	// address are the same endpoint, so each needs its own port to survive
	// deduplication.
	listen := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = l.Close() })
		acceptLoop(l)
		return l.Addr().(*net.TCPAddr).Port
	}
	port3 := listen()
	port4 := listen()

	vmessJSON := fmt.Sprintf(`{"add":"127.0.0.1","port":%d,"ps":"DE Frankfurt 03","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"tcp","type":"none"}`, port)
	plain := strings.Join([]string{
		fmt.Sprintf("vless://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee@127.0.0.1:%d?type=tcp&security=none#US Ashburn 01", port),
		fmt.Sprintf("trojan://secret@127.0.0.1:%d#NL Amsterdam 02", port),
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessJSON)),
		"this-is-not-a-uri",
		// Same label as the first line, different address.
		fmt.Sprintf("vless://bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee@127.0.0.1:%d?type=tcp&security=none#US Ashburn 01", port2),
	}, "\n")
	b64 := base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("vless://aaaaaaaa-bbbb-cccc-dddd-ffffffffffff@127.0.0.1:%d?type=tcp#JP Tokyo 05", port3),
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
		"subscriptions":            srvPlain.URL + "\n" + srvB64.URL + "\nvless://aaaaaaaa-bbbb-cccc-dddd-999999999999@127.0.0.1:" + fmt.Sprint(port4) + "#Inline 09",
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
	if !strings.Contains(data, `"k":"Endpoints","v":"5"`) {
		t.Errorf("status does not report 5 endpoints: %s", data)
	}
	if !strings.Contains(data, `"k":"Local ports"`) {
		t.Errorf("status does not report the local port range: %s", data)
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
	// One row per distinct endpoint, so exactly the kept ones light up —
	// not every repetition of a kept label or address.
	var srvPayload struct {
		Servers []struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			Selected bool   `json:"selected"`
		} `json:"servers"`
	}
	if err := json.Unmarshal([]byte(servers), &srvPayload); err != nil {
		t.Fatalf("servers json: %v (%s)", err, servers)
	}
	if len(srvPayload.Servers) != 5 {
		t.Errorf("endpoint list has %d rows, want 5 distinct endpoints", len(srvPayload.Servers))
	}
	seenRows := map[string]bool{}
	selectedRows := 0
	for _, row := range srvPayload.Servers {
		rowKey := row.Protocol + "|" + row.Host + "|" + strconv.Itoa(row.Port)
		if seenRows[rowKey] {
			t.Errorf("duplicate row %q in the endpoint list", rowKey)
		}
		seenRows[rowKey] = true
		if row.Selected {
			selectedRows++
		}
	}
	if selectedRows != 2 {
		t.Errorf("endpoint list marks %d selected, want 2", selectedRows)
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
	var emitted struct {
		Candidates []vpnsCandidate `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(candidates), &emitted); err != nil {
		t.Fatalf("candidates json: %v (%s)", err, candidates)
	}
	// node_count is 2, so the gateway must see exactly two exits.
	if len(emitted.Candidates) != 2 {
		t.Errorf("got %d candidates, want 2 (node_count): %s", len(emitted.Candidates), candidates)
	}
	seenNames := map[string]bool{}
	for _, c := range emitted.Candidates {
		// The gateway keys exit health by name: without the address two
		// same-labelled nodes collapse into one exit.
		if !strings.Contains(c.Name, " @ ") || !strings.Contains(c.Name, ":") {
			t.Errorf("exit name %q does not carry its address: %s", c.Name, candidates)
		}
		if seenNames[c.Name] {
			t.Errorf("duplicate exit name %q: %s", c.Name, candidates)
		}
		seenNames[c.Name] = true
		if !strings.HasPrefix(c.Proxy, "socks5://127.0.0.1:") {
			t.Errorf("exit proxy = %q, want a local socks port", c.Proxy)
		}
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
	// The addresses ride along so two endpoints sharing one label stay two
	// endpoints (they are what the exit name and the "selected" flag key on).
	if len(first.SelectedKeys) != len(first.Selected) {
		t.Errorf("selected = %v, selected_keys = %v, want one address per kept IP", first.Selected, first.SelectedKeys)
	}
	seenLabels := map[string]bool{}
	for i, name := range first.Selected {
		if seenLabels[name] {
			t.Errorf("selected labels are not unique: %v", first.Selected)
		}
		seenLabels[name] = true
		if strings.Count(first.SelectedKeys[i], "|") != 2 {
			t.Errorf("selected_keys[%d] = %q, want a protocol|host|port identity", i, first.SelectedKeys[i])
		}
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

	// Keeping more IPs than the subscription has distinct labels has to fill
	// the remaining slots by address instead of dropping them: the duplicated
	// US label stays in twice, behind two different addresses.
	keepAll := map[string]string{}
	for k, v := range edited {
		keepAll[k] = v
	}
	keepAll["node_count"] = "9"
	// One base port: the range must size itself to the kept count instead of
	// collapsing several exits onto one port.
	keepAll["local_socks_ports"] = "1080"
	if _, err := a.CallString("Data", vpnsPayload(dir, "vllm-zen", "", keepAll)); err != nil {
		t.Fatalf("Data (keep all): %v", err)
	}
	time.Sleep(6 * time.Second)
	fourth := readVpnState(t, dir)
	if !fourth.LastRefresh.After(third.LastRefresh) {
		t.Errorf("node_count=9 did not trigger a refresh: %v -> %v", third.LastRefresh, fourth.LastRefresh)
	}
	if len(fourth.Selected) != 5 {
		t.Errorf("selected = %v, want all 5 endpoints kept", fourth.Selected)
	}
	duplicated := 0
	for _, name := range fourth.Selected {
		if name == "US Ashburn 01" {
			duplicated++
		}
	}
	if duplicated != 2 {
		t.Errorf("selected = %v, want the repeated label kept twice", fourth.Selected)
	}
	if len(fourth.SelectedKeys) != len(fourth.Selected) {
		t.Errorf("selected = %v, selected_keys = %v, want one address per entry", fourth.Selected, fourth.SelectedKeys)
	}
	seenKeys := map[string]bool{}
	for _, key := range fourth.SelectedKeys {
		if seenKeys[key] {
			t.Errorf("duplicate selected identity %q in %v", key, fourth.SelectedKeys)
		}
		seenKeys[key] = true
	}

	// Auto-sized ports: base 1080 with one port per kept endpoint.
	autoCandidates, err := a.CallString("EgressCandidates", vpnsPayload(dir, "vllm-zen", "", keepAll))
	if err != nil {
		t.Fatalf("EgressCandidates (auto ports): %v", err)
	}
	var auto struct {
		Candidates []vpnsCandidate `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(autoCandidates), &auto); err != nil {
		t.Fatalf("auto candidates json: %v (%s)", err, autoCandidates)
	}
	if len(auto.Candidates) != len(fourth.Selected) {
		t.Fatalf("got %d candidates, want %d kept endpoints: %s", len(auto.Candidates), len(fourth.Selected), autoCandidates)
	}
	seenPorts := map[string]bool{}
	for _, c := range auto.Candidates {
		if !strings.HasPrefix(c.Proxy, "socks5://127.0.0.1:") {
			t.Errorf("proxy = %q, want a local socks port", c.Proxy)
			continue
		}
		seenPorts[strings.TrimPrefix(c.Proxy, "socks5://127.0.0.1:")] = true
	}
	if len(seenPorts) != len(fourth.Selected) {
		t.Errorf("distinct local ports = %d (%v), want one per kept endpoint (%d)", len(seenPorts), seenPorts, len(fourth.Selected))
	}
	for port := 1080; port < 1080+len(fourth.Selected); port++ {
		if !seenPorts[strconv.Itoa(port)] {
			t.Errorf("auto ports = %v, want the range starting at 1080", seenPorts)
			break
		}
	}
}
