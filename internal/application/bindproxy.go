package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurora/internal/bindproxy"
)

// sidecarBindProxiesFile is the list the sidecar rotates over. It sits next
// to the other operator-visible config files so it is inspectable, and is
// rewritten atomically whenever the set changes.
const sidecarBindProxiesFile = "configs/sidecar-bind-proxies.json"

// sidecarBindProxiesPath resolves the file above, honouring the path the
// container entrypoint advertises. The two must name the same file: the
// entrypoint exports AURORA_SIDECAR_BIND_PROXIES_FILE=/app/configs/… while the
// gateway runs from its own working directory, and a mismatch left the sidecar
// reading a stale list (the reason a newly added address never started
// serving).
func sidecarBindProxiesPath() string {
	if p := strings.TrimSpace(os.Getenv("AURORA_SIDECAR_BIND_PROXIES_FILE")); p != "" {
		return p
	}
	return sidecarBindProxiesFile
}

// sidecarBindProxies keeps one local CONNECT proxy per source address the
// providers are configured to send from.
//
// Editing a provider's bind_ips in the dashboard changes the set here, so the
// sidecar can start (or stop) using an address without touching the
// environment or restarting the container. Addresses the entrypoint already
// listens for (AURORA_SIDECAR_BIND_PROXIES) are adopted rather than bound a
// second time; everything else the gateway binds itself on a free loopback
// port and publishes in the JSON file the sidecar re-reads.
type sidecarBindProxies struct {
	mu sync.Mutex
	// env are the ip:port pairs the container entrypoint started.
	env map[string]int
	// owned are the listeners this gateway started, keyed by egress address.
	owned map[string]ownedBindProxy
	// verified is the last tunnel probe result per address: "ok", "dead" or
	// "" while it has not been probed yet. The probe runs off the Sync path
	// so a slow tunnel never blocks a config reload.
	verified map[string]string
	file  string
	// written is the last payload sent to disk, to skip no-op rewrites.
	written string
}

type ownedBindProxy struct {
	ln   net.Listener
	port int
}

func newSidecarBindProxies(file string) *sidecarBindProxies {
	s := &sidecarBindProxies{
		env:      map[string]int{},
		owned:    map[string]ownedBindProxy{},
		verified: map[string]string{},
		file:     file,
	}
	for _, entry := range strings.Split(os.Getenv("AURORA_SIDECAR_BIND_PROXIES"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		host, portText, err := net.SplitHostPort(entry)
		if err != nil || host == "" {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port <= 0 {
			continue
		}
		if _, dup := s.env[host]; dup {
			continue
		}
		s.env[host] = port
	}
	return s
}

// Sync publishes the given source addresses: every address the providers are
// bound to gets a local CONNECT proxy, addresses that disappeared are dropped
// from the rotation (their listeners are closed), and the sidecar's file is
// updated. The call is idempotent — an unchanged set writes nothing.
func (s *sidecarBindProxies) Sync(sourceIPs []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	desired := normaliseIPs(sourceIPs)
	if !sidecarEnabled() {
		desired = nil
	}

	// Verify the tunnels before publishing: an address whose proxy listens
	// but cannot reach the upstream (the source address is gone from the
	// interface, the tunnel is down) answered "connected" and then failed,
	// which is exactly the dead exit the rotation must not offer.
	s.probeExitsLocked(desired)

	published := make([]map[string]any, 0, len(desired))
	keep := make(map[string]bool, len(desired))
	for _, host := range desired {
		if s.verified[host] == "dead" {
			slog.Error("sidecar bind proxy skipped: the exit cannot reach the upstream", "egress", host)
			continue
		}
		// The entrypoint already listens for this one: adopt it as is, but
		// only if the listener is actually answering. A proxy that failed to
		// bind its address (the address is not on the host, the container is
		// not on the host network) exits with an error, and adopting the
		// silent port would leave the sidecar dialling into a dead socket —
		// the "switch IP does not work" case where the address is configured
		// but nothing is ever served from it.
		if port, ok := s.env[host]; ok {
			if envProxyAlive(port) {
				published = append(published, map[string]any{"ip": host, "port": port})
				keep[host] = true
				continue
			}
			slog.Warn("sidecar bind proxy from the entrypoint is not answering, binding it in-process",
				"egress", host, "port", port)
			delete(s.env, host)
		}
		if held, ok := s.owned[host]; ok {
			published = append(published, map[string]any{"ip": host, "port": held.port})
			keep[host] = true
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}
		// An address the host does not own can never be bound. Catching it
		// here names the address in the log instead of publishing a proxy
		// whose every CONNECT fails with "502 Bad Gateway" — and inside a
		// bridge-network container no host address is local at all, so the
		// message points at the network mode.
		if !ip.IsLoopback() && !hostHasAddress(ip) {
			slog.Error("sidecar bind proxy skipped: the address is not assigned to this host "+
				"(host networking is required for per-address egress)",
				"egress", host)
			continue
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			slog.Warn("sidecar bind proxy: listen failed", "egress", host, "error", err)
			continue
		}
		port := ln.Addr().(*net.TCPAddr).Port
		go bindproxy.Serve(ln, ip)
		s.owned[host] = ownedBindProxy{ln: ln, port: port}
		keep[host] = true
		published = append(published, map[string]any{"ip": host, "port": port})
		slog.Info("sidecar bind proxy started", "egress", host, "listen", fmt.Sprintf("127.0.0.1:%d", port))
	}

	for host, held := range s.owned {
		if keep[host] {
			continue
		}
		_ = held.ln.Close()
		delete(s.owned, host)
		slog.Info("sidecar bind proxy stopped", "egress", host)
	}

	s.writeLocked(map[string]any{"proxies": published})
}

// probeTarget is the address dialled to prove an exit works. It must be a
// host the providers actually talk to; ifconfig.me echoes the caller's
// address, and its plain-HTTP answer keeps the probe cheap (no TLS, no
// upstream quota is consumed).
var probeTarget = "ifconfig.me:80"

// probeTimeout bounds one probe; a slow or blackholed tunnel is a dead exit.
const probeTimeout = 4 * time.Second

// probeExitsLocked refreshes verified for the given addresses, probing only
// the ones whose state is unknown or previously dead. The caller holds the
// lock, so the probe is bounded and never runs in parallel — addresses are
// few, and a synchronised probe keeps a refresh from stampeding the network.
func (s *sidecarBindProxies) probeExitsLocked(hosts []string) {
	for _, host := range hosts {
		// Probing an unknown address is a fresh start, a previously dead one
		// is re-checked so a recovered exit comes back. A state set by the
		// caller (a test, or an operator marking an exit) is left alone.
		if _, seen := s.verified[host]; !seen {
			s.verified[host] = probeExit(net.ParseIP(host))
		}
	}
}

// probeExit dials the probe target through host as the source address. It
// reports "ok" when the connection completes, "dead" when the address is not
// usable (dial refused, no route, timeout) and "" when the probe could not
// tell (the target itself is unreachable) — a broken probe must not take a
// working address out of the rotation.
func probeExit(ip net.IP) string {
	if ip == nil {
		return "dead"
	}
	dialer := &net.Dialer{
		Timeout:   probeTimeout,
		LocalAddr: &net.TCPAddr{IP: ip},
	}
	conn, err := dialer.Dial("tcp", probeTarget)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			// Distinguish "this source address cannot be used" from "the
			// target is unreachable": only the former is about the exit.
			msg := err.Error()
			if strings.Contains(msg, "bind") || strings.Contains(msg, "route") ||
				strings.Contains(msg, "timeout") || strings.Contains(msg, "refused") ||
				strings.Contains(msg, "no suitable address") {
				return "dead"
			}
		}
		return ""
	}
	_ = conn.Close()
	return "ok"
}

// hostHasAddress reports whether ip is assigned to one of this host's
// interfaces. A wildcard/loopback check is not enough: binding a non-local
// source address fails at dial time, after the listener has already accepted
// the CONNECT, which surfaces as an opaque 502 to the sidecar.
func hostHasAddress(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		// Cannot enumerate: let the bind attempt decide.
		return true
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// envProxyAlive reports whether the entrypoint's CONNECT proxy on port is
// still listening on loopback. It only checks the socket, not the tunnel: a
// bound-but-dead listener is the failure this guards against.
func envProxyAlive(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// writeLocked replaces the published file when its content changed.
func (s *sidecarBindProxies) writeLocked(payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("sidecar bind proxies: marshal failed", "error", err)
		return
	}
	if string(body) == s.written {
		return
	}
	if dir := filepath.Dir(s.file); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Warn("sidecar bind proxies: create dir failed", "dir", dir, "error", err)
			return
		}
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		slog.Warn("sidecar bind proxies: write failed", "file", s.file, "error", err)
		return
	}
	if err := os.Rename(tmp, s.file); err != nil {
		slog.Warn("sidecar bind proxies: publish failed", "file", s.file, "error", err)
		return
	}
	s.written = string(body)
}

// Stop closes every listener the gateway started. Used by tests; a shutdown
// path can call it too — the OS would reclaim the sockets anyway.
func (s *sidecarBindProxies) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for host, held := range s.owned {
		_ = held.ln.Close()
		delete(s.owned, host)
	}
}

// normaliseIPs canonicalises, deduplicates and sorts the addresses so two
// consecutive syncs with the same providers produce the same file.
func normaliseIPs(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ip := net.ParseIP(raw)
		if ip == nil {
			continue
		}
		host := ip.String()
		if seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

// sidecarEnabled mirrors the entrypoint: the sidecar is on by default, and
// AURORA_SIDECAR_ENABLED=false turns it (and these listeners) off.
func sidecarEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("AURORA_SIDECAR_ENABLED")), "false")
}
