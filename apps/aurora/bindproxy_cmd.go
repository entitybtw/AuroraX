package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

// runBindProxySubcommand implements `aurora bindproxy`, a minimal HTTP CONNECT
// proxy that binds every outbound connection to a specific local IP.
//
// The sidecar runs on Bun because some upstreams fingerprint the TLS
// handshake, but Bun's fetch ignores a source address. This proxy lets
// the sidecar dial through a chosen egress IP while Bun still performs the
// TLS handshake end-to-end (the proxy only relays bytes).
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

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("bindproxy: accept: %v", err)
			continue
		}
		go serveBindProxyConn(conn, parsed)
	}
}

func serveBindProxyConn(conn net.Conn, bindIP net.IP) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil || req.Method != http.MethodConnect {
		_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	dialer := &net.Dialer{LocalAddr: &net.TCPAddr{IP: bindIP}}
	upstream, err := dialer.Dial("tcp", req.Host)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstream.Close()

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, upstream); done <- struct{}{} }()
	<-done
}
