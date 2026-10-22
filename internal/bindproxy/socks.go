package bindproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/proxy"
)

// Bridges turn a SOCKS5 exit into something a sidecar can use.
//
// The sidecar runs on Bun, whose fetch speaks HTTP proxies but not SOCKS5,
// while the VPN extension contributes SOCKS5 exits. Each exit therefore gets
// a local HTTP CONNECT proxy that dials the target through that exit: the
// sidecar keeps using plain HTTP proxies, and the connection still leaves
// through the tunnel.
//
// A bridge is shared by every request that uses the same exit, so the exits
// are cached per SOCKS URL and the listeners are closed on Stop.
var (
	bridgesMu sync.Mutex
	bridges   = map[string]*bridge{}
)

type bridge struct {
	ln     net.Listener
	proxy  string // the http:// URL the sidecar connects to
	target *url.URL
}

// EnsureBridge returns the local HTTP CONNECT proxy that forwards through the
// given SOCKS5 exit, starting it on first use. The second return value is
// false when the URL is not a SOCKS5 proxy this bridge can serve.
func EnsureBridge(socksURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(socksURL))
	if err != nil || u.Host == "" {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
	default:
		return "", false
	}

	bridgesMu.Lock()
	defer bridgesMu.Unlock()
	if b, ok := bridges[u.String()]; ok {
		return b.proxy, true
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("bindproxy: bridge listen failed: %v", err)
		return "", false
	}
	port := ln.Addr().(*net.TCPAddr).Port
	b := &bridge{
		ln:     ln,
		proxy:  fmt.Sprintf("http://127.0.0.1:%d", port),
		target: u,
	}
	bridges[u.String()] = b
	go serveBridge(ln, u)
	return b.proxy, true
}

// StopBridges closes every bridge listener. Used by tests; a shutdown path
// can call it too — the OS would reclaim the sockets anyway.
func StopBridges() {
	bridgesMu.Lock()
	defer bridgesMu.Unlock()
	for key, b := range bridges {
		_ = b.ln.Close()
		delete(bridges, key)
	}
}

// serveBridge accepts CONNECT requests and opens the tunnel through the
// SOCKS5 exit behind it.
func serveBridge(ln net.Listener, socks *url.URL) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("bindproxy: bridge accept: %v", err)
			continue
		}
		go handleBridge(conn, socks)
	}
}

// handleBridge relays one client connection to the SOCKS5 exit.
//
// Two shapes are served, because HTTP clients use both depending on the
// scheme of the target: a CONNECT request opens a tunnel (https), while a
// plain request is forwarded in origin form over the same tunnel (http).
func handleBridge(conn net.Conn, socks *url.URL) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	if req.Method == http.MethodConnect {
		handleBridgeConnect(conn, br, socks, req.Host)
		return
	}
	handleBridgeForward(conn, br, socks, req)
}

// handleBridgeConnect opens a tunnel through the exit and relays bytes.
func handleBridgeConnect(conn net.Conn, br *bufio.Reader, socks *url.URL, target string) {
	upstream, err := dialThroughSocks(socks, target)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstream.Close()

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// Wait for the upstream->client direction: the upstream closes when it is
	// done answering, and returning earlier would truncate the response the
	// caller is about to read. The other direction ends when the client
	// closes its side.
	upstreamDone := make(chan struct{})
	clientDone := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, br); close(clientDone) }()
	go func() { _, _ = io.Copy(conn, upstream); close(upstreamDone) }()
	select {
	case <-upstreamDone:
	case <-clientDone:
	}
}

// handleBridgeForward sends a plain request through the exit and copies the
// answer back.
func handleBridgeForward(conn net.Conn, br *bufio.Reader, socks *url.URL, req *http.Request) {
	target := req.URL.Host
	if target == "" {
		target = req.Host
	}
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if !strings.Contains(target, ":") {
		target += ":80"
	}

	upstream, err := dialThroughSocks(socks, target)
	if err != nil {
		_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstream.Close()

	// The request is written in origin form: the tunnel already carries the
	// hop to the target, so the proxy address must not be repeated.
	outReq := req.Clone(req.Context())
	outReq.RequestURI = ""
	if err := outReq.Write(upstream); err != nil {
		return
	}
	if req.Body != nil {
		_, _ = io.Copy(upstream, req.Body)
	}

	upstreamDone := make(chan struct{})
	clientDone := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, br); close(clientDone) }()
	go func() { _, _ = io.Copy(conn, upstream); close(upstreamDone) }()
	select {
	case <-upstreamDone:
	case <-clientDone:
	}
}

// dialThroughSocks opens a tunnel to target through the SOCKS5 exit.
func dialThroughSocks(socks *url.URL, target string) (net.Conn, error) {
	var auth *proxy.Auth
	if socks.User != nil {
		auth = &proxy.Auth{User: socks.User.Username()}
		if pass, ok := socks.User.Password(); ok {
			auth.Password = pass
		}
	}
	dialer, err := proxy.SOCKS5("tcp", socks.Host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		return cd.DialContext(context.Background(), "tcp", target)
	}
	return dialer.Dial("tcp", target)
}
