package main

import (
	"fmt"
	"io"
	"net"
	"os"

	"aurora/internal/bindproxy"
)

// runBindProxySubcommand implements `aurora bindproxy`, a minimal HTTP CONNECT
// proxy that binds every outbound connection to a specific local IP.
//
// The sidecar runs on Bun because some upstreams fingerprint the TLS
// handshake, but Bun's fetch ignores a source address. This proxy lets
// the sidecar dial through a chosen egress IP while Bun still performs the
// TLS handshake end-to-end (the proxy only relays bytes).
//
// The gateway starts the same proxy in-process for any address a provider
// gains at runtime (see internal/application/bindproxy.go); this subcommand
// is the entrypoint's static variant.
//
// Environment:
//
//	BIND_IP       local IP to bind outbound sockets to (required)
//	PROXY_LISTEN  listen address, e.g. 127.0.0.1:8981 (required)
func runBindProxySubcommand(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 || args[0] != "bindproxy" {
		return false, 0
	}

	bindIP := os.Getenv("BIND_IP")
	listen := os.Getenv("PROXY_LISTEN")
	if bindIP == "" || listen == "" {
		fmt.Fprintln(stderr, "bindproxy: BIND_IP and PROXY_LISTEN are required")
		return true, 2
	}
	parsed := net.ParseIP(bindIP)
	if parsed == nil {
		fmt.Fprintf(stderr, "bindproxy: invalid BIND_IP %q\n", bindIP)
		return true, 2
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		fmt.Fprintf(stderr, "bindproxy: listen %s: %v\n", listen, err)
		return true, 1
	}
	fmt.Fprintf(stdout, "bindproxy listening on %s (egress %s)\n", listen, bindIP)
	bindproxy.Serve(ln, parsed)
	return true, 0
}
