package httpclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

type utlsRoundTripper struct {
	bindAddr string
	// proxy, when set, tunnels the connection before the TLS handshake so a
	// fingerprint-preserving client can still leave through another exit.
	proxy *url.URL
}

func (rt *utlsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(host, port)

	conn, err := rt.dial(req.Context(), addr)
	if err != nil {
		return nil, fmt.Errorf("utls: tcp dial %s: %w", addr, err)
	}

	tlsConfig := &utls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	uconn := utls.UClient(conn, tlsConfig, utls.HelloChrome_120)
	if err := uconn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("utls: handshake %s: %w", addr, err)
	}

	log.Printf("utls: RoundTrip called: %s %s (bind=%s)", req.Method, req.URL, rt.bindAddr)

	proto := uconn.ConnectionState().NegotiatedProtocol
	if proto == "h2" || proto == "" {
		return rt.doHTTP2(uconn, req)
	}

	return rt.doHTTP1(uconn, req)
}

// dial establishes the transport connection for this exit: optionally bound
// to a local source address, optionally through a proxy.
func (rt *utlsRoundTripper) dial(ctx context.Context, addr string) (net.Conn, error) {
	forward := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if rt.bindAddr != "" {
		forward.LocalAddr = &net.TCPAddr{IP: net.ParseIP(rt.bindAddr)}
	}
	if rt.proxy == nil {
		return forward.DialContext(ctx, "tcp", addr)
	}
	return dialViaProxy(ctx, forward, rt.proxy, addr)
}

// dialViaProxy opens a tunnel to addr through p. SOCKS5 (with and without
// remote DNS) is delegated to x/net/proxy; http/https proxies are handled
// with an explicit CONNECT so the local bind address is preserved.
func dialViaProxy(ctx context.Context, forward *net.Dialer, p *url.URL, target string) (net.Conn, error) {
	switch strings.ToLower(p.Scheme) {
	case "socks5", "socks5h":
		d, err := proxy.FromURL(p, forward)
		if err != nil {
			return nil, err
		}
		if cd, ok := d.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, "tcp", target)
		}
		return d.Dial("tcp", target)
	case "http", "https", "":
		return dialHTTPConnect(ctx, forward, p, target)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", p.Scheme)
	}
}

func dialHTTPConnect(ctx context.Context, forward *net.Dialer, p *url.URL, target string) (net.Conn, error) {
	proxyHost := p.Hostname()
	proxyPort := p.Port()
	if proxyPort == "" {
		if strings.EqualFold(p.Scheme, "https") {
			proxyPort = "443"
		} else {
			proxyPort = "80"
		}
	}
	conn, err := forward.DialContext(ctx, "tcp", net.JoinHostPort(proxyHost, proxyPort))
	if err != nil {
		return nil, fmt.Errorf("proxy dial %s: %w", proxyHost, err)
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = conn.Close()
		}
	}()

	if strings.EqualFold(p.Scheme, "https") {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyHost, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("proxy tls handshake: %w", err)
		}
		conn = tlsConn
	}

	var sb strings.Builder
	sb.WriteString("CONNECT ")
	sb.WriteString(target)
	sb.WriteString(" HTTP/1.1\r\nHost: ")
	sb.WriteString(target)
	sb.WriteString("\r\n")
	if u := p.User; u != nil {
		password, _ := u.Password()
		token := base64.StdEncoding.EncodeToString([]byte(u.Username() + ":" + password))
		sb.WriteString("Proxy-Authorization: Basic ")
		sb.WriteString(token)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	if _, err := conn.Write([]byte(sb.String())); err != nil {
		return nil, fmt.Errorf("proxy connect write: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return nil, fmt.Errorf("proxy connect read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy connect %s: %s", target, resp.Status)
	}
	closeOnError = false
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufferedConn replays bytes the proxy reader consumed past the CONNECT
// response, which would otherwise be lost to the TLS handshake.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func (rt *utlsRoundTripper) doHTTP1(uconn *utls.UConn, req *http.Request) (*http.Response, error) {
	if err := writeHTTPRequest(uconn, req); err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls: write request: %w", err)
	}

	resp, err := readHTTPResponse(uconn, req)
	if err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls: read response: %w", err)
	}

	resp.Body = &utlsBody{ReadCloser: resp.Body, conn: uconn}
	return resp, nil
}

func (rt *utlsRoundTripper) doHTTP2(uconn *utls.UConn, req *http.Request) (*http.Response, error) {
	tr := &http2.Transport{}
	cc, err := tr.NewClientConn(uconn)
	if err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls h2: new client conn: %w", err)
	}

	resp, err := cc.RoundTrip(req)
	if err != nil {
		cc.Close()
		uconn.Close()
		return nil, fmt.Errorf("utls h2: round trip: %w", err)
	}

	resp.Body = &h2ClientBody{ReadCloser: resp.Body, cc: cc, conn: uconn}
	return resp, nil
}

type h2ClientBody struct {
	io.ReadCloser
	cc   *http2.ClientConn
	conn net.Conn
}

func (b *h2ClientBody) Close() error {
	err := b.ReadCloser.Close()
	b.cc.Close()
	return err
}

func NewUTLSTransport() http.RoundTripper {
	return &utlsRoundTripper{}
}

func NewUTLSHTTPClient() *http.Client {
	return &http.Client{
		Transport: NewUTLSTransport(),
		Timeout:   600 * time.Second,
	}
}

func NewUTLSHTTPClientWithBindIP(bindIP string) *http.Client {
	return &http.Client{
		Transport: &utlsRoundTripper{bindAddr: bindIP},
		Timeout:   600 * time.Second,
	}
}

func writeHTTPRequest(conn net.Conn, req *http.Request) error {
	var sb strings.Builder
	sb.WriteString(req.Method)
	sb.WriteByte(' ')
	if req.URL.RawPath != "" {
		sb.WriteString(req.URL.RawPath)
	} else {
		sb.WriteString(req.URL.RequestURI())
	}
	sb.WriteString(" HTTP/1.1\r\n")
	sb.WriteString("Host: ")
	sb.WriteString(req.URL.Host)
	sb.WriteString("\r\n")

	if req.Header.Get("Connection") == "" {
		sb.WriteString("Connection: keep-alive\r\n")
	}
	for key, vals := range req.Header {
		for _, val := range vals {
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(val)
			sb.WriteString("\r\n")
		}
	}
	sb.WriteString("\r\n")

	_, err := conn.Write([]byte(sb.String()))
	return err
}

func readHTTPResponse(conn net.Conn, req *http.Request) (*http.Response, error) {
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type utlsBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *utlsBody) Close() error {
	err := b.ReadCloser.Close()
	b.conn.Close()
	return err
}

var _ http.RoundTripper = (*utlsRoundTripper)(nil)
