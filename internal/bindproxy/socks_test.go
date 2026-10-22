package bindproxy

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// socks5Echo starts a minimal SOCKS5 server that answers every CONNECT with
// a tunnel to the requested address and echoes the bytes it receives.
func socks5Echo(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				// Greeting: version, method count, methods.
				head, err := br.ReadByte()
				if err != nil || head != 0x05 {
					return
				}
				count, err := br.ReadByte()
				if err != nil {
					return
				}
				if _, err := io.CopyN(io.Discard, br, int64(count)); err != nil {
					return
				}
				if _, err := c.Write([]byte{0x05, 0x00}); err != nil {
					return
				}
				// Request: version, command, reserved, address type.
				req := make([]byte, 4)
				if _, err := io.ReadFull(br, req); err != nil {
					return
				}
				var host string
				switch req[3] {
				case 0x01:
					addr := make([]byte, 4)
					if _, err := io.ReadFull(br, addr); err != nil {
						return
					}
					host = net.IP(addr).String()
				case 0x03:
					length, err := br.ReadByte()
					if err != nil {
						return
					}
					name := make([]byte, length)
					if _, err := io.ReadFull(br, name); err != nil {
						return
					}
					host = string(name)
				default:
					return
				}
				portBytes := make([]byte, 2)
				if _, err := io.ReadFull(br, portBytes); err != nil {
					return
				}
				port := binary.BigEndian.Uint16(portBytes)

				upstream, err := net.Dial("tcp", net.JoinHostPort(host, itoa(int(port))))
				if err != nil {
					_, _ = c.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
					return
				}
				defer upstream.Close()
				if _, err := c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
					return
				}
				done := make(chan struct{}, 2)
				go func() { _, _ = io.Copy(upstream, br); done <- struct{}{} }()
				go func() { _, _ = io.Copy(c, upstream); done <- struct{}{} }()
				<-done
			}(conn)
		}
	}()

	return "socks5://" + ln.Addr().String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [6]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// A SOCKS5 exit becomes a local HTTP CONNECT proxy, so a client that only
// speaks HTTP proxies (Bun's fetch) can still leave through the tunnel.
func TestEnsureBridgeTunnelsThroughSocksExit(t *testing.T) {
	StopBridges()
	t.Cleanup(StopBridges)

	socks := socks5Echo(t)

	// The target the bridge is asked to reach: a plain HTTP server, so the
	// response the client reads is a real one.
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen target: %v", err)
	}
	defer target.Close()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				_, _ = http.ReadRequest(br)
				body := "tunnelled"
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: " +
					itoa(len(body)) + "\r\nConnection: close\r\n\r\n" + body))
			}(conn)
		}
	}()

	bridge, ok := EnsureBridge(socks)
	if !ok {
		t.Fatal("expected the SOCKS5 exit to be bridged")
	}
	proxyURL, err := url.Parse(bridge)
	if err != nil {
		t.Fatalf("parse bridge url: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	resp, err := client.Get("http://" + target.Addr().String() + "/")
	if err != nil {
		t.Fatalf("request through bridge: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "tunnelled" {
		t.Fatalf("body = %q, want %q", body, "tunnelled")
	}
}

// The same exit is bridged once and reused: the sidecar and the gateway then
// share one listener per exit.
func TestEnsureBridgeReusesTheSameExit(t *testing.T) {
	StopBridges()
	t.Cleanup(StopBridges)

	socks := socks5Echo(t)
	first, ok := EnsureBridge(socks)
	if !ok {
		t.Fatal("expected the SOCKS5 exit to be bridged")
	}
	second, ok := EnsureBridge(socks)
	if !ok {
		t.Fatal("expected the SOCKS5 exit to be bridged")
	}
	if first != second {
		t.Fatalf("bridge url changed: %q then %q", first, second)
	}
}

// A URL that is not a SOCKS5 exit is not bridged: it is used as it is.
func TestEnsureBridgeIgnoresOtherSchemes(t *testing.T) {
	StopBridges()
	t.Cleanup(StopBridges)

	if _, ok := EnsureBridge("http://127.0.0.1:8080"); ok {
		t.Fatal("an http proxy must not be bridged")
	}
	if _, ok := EnsureBridge("not a url"); ok {
		t.Fatal("a malformed url must not be bridged")
	}
}
