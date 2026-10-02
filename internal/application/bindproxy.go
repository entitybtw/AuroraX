package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"aurora/internal/bindproxy"
)

// sidecarBindProxiesFile is the list the sidecar rotates over. It sits next
// to the other operator-visible config files so it is inspectable, and is
// rewritten atomically whenever the set changes.
const sidecarBindProxiesFile = "configs/sidecar-bind-proxies.json"

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
		env:   map[string]int{},
		owned: map[string]ownedBindProxy{},
		file:  file,
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

	published := make([]map[string]any, 0, len(desired))
	keep := make(map[string]bool, len(desired))
	for _, host := range desired {
		// The entrypoint already listens for this one: adopt it as is.
		if port, ok := s.env[host]; ok {
			published = append(published, map[string]any{"ip": host, "port": port})
			keep[host] = true
			continue
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
