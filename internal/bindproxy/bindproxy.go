// Package bindproxy serves a minimal HTTP CONNECT proxy that binds every
// outbound connection to one chosen local address.
//
// The sidecar runs on Bun because some upstreams fingerprint the TLS
// handshake, but Bun's fetch cannot pick a source address. This proxy only
// relays bytes, so the sidecar can dial through a chosen egress IP while Bun
// still performs the TLS handshake end-to-end.
package bindproxy

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
)

// Serve accepts CONNECT requests on ln until it is closed, dialing upstream
// with bindIP as the source address of every outbound connection.
func Serve(ln net.Listener, bindIP net.IP) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("bindproxy: accept: %v", err)
			continue
		}
		go Handle(conn, bindIP)
	}
}

// Handle relays one client connection: a CONNECT request in, a tunnel out
// that originates from bindIP. Unknown methods are rejected outright.
func Handle(conn net.Conn, bindIP net.IP) {
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
