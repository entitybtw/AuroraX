package httpclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	// preset names the ClientHello to reproduce. The zero value means the
	// historical default (Chrome 120), so every existing caller keeps the
	// fingerprint it had.
	preset string
	// insecureSkipVerify is only used by tests against a self-signed server.
	insecureSkipVerify bool
}

// UTLSClientHello resolves a preset name to the ClientHello it reproduces.
func UTLSClientHello(preset string) (utls.ClientHelloID, bool) {
	id, ok := utlsPresets[strings.ToLower(strings.TrimSpace(preset))]
	return id, ok
}

// UTLSPresets lists the fingerprint presets an operator can rotate through.
func UTLSPresets() []string {
	names := make([]string, 0, len(utlsPresets))
	for name := range utlsPresets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// utlsPresets maps a stable configuration name to a uTLS ClientHello. The
// names are what a provider config or an environment variable carries; the
// values are the fingerprints they stand for.
var utlsPresets = map[string]utls.ClientHelloID{
	"chrome":          utls.HelloChrome_Auto,
	"chrome120":       utls.HelloChrome_120,
	"chrome120pq":     utls.HelloChrome_120_PQ,
	"chrome115pq":     utls.HelloChrome_115_PQ,
	"firefox":         utls.HelloFirefox_Auto,
	"firefox120":      utls.HelloFirefox_120,
	"safari":          utls.HelloSafari_Auto,
	"ios":             utls.HelloIOS_Auto,
	"edge":            utls.HelloEdge_Auto,
	"android":         utls.HelloAndroid_11_OkHttp,
	"360":             utls.Hello360_Auto,
	"qq":              utls.HelloQQ_Auto,
	"randomized":      utls.HelloRandomizedALPN,
	"randomized-alpn": utls.HelloRandomizedALPN,
}

// clientHello returns the ClientHello for this transport, defaulting to the
// long-standing Chrome 120 fingerprint.
func (rt *utlsRoundTripper) clientHello() utls.ClientHelloID {
	if id, ok := UTLSClientHello(rt.preset); ok {
		return id
	}
	return utls.HelloChrome_120
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
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: rt.insecureSkipVerify,
	}

	uconn := utls.UClient(conn, tlsConfig, rt.clientHello())
	if err := uconn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("utls: handshake %s: %w", addr, err)
	}

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

// UTLSOptions configures a fingerprint-preserving transport: where it dials
// from, what it tunnels through, and which ClientHello it reproduces.
type UTLSOptions struct {
	// BindAddr is a local source address to dial from.
	BindAddr string
	// Proxy tunnels the connection before the TLS handshake.
	Proxy *url.URL
	// Preset selects the ClientHello (see UTLSPresets). Empty means the
	// default Chrome 120 fingerprint.
	Preset string
	// InsecureSkipVerify disables certificate verification. Tests only.
	InsecureSkipVerify bool
}

// NewUTLSTransportWithOptions builds a transport for one exit and one
// fingerprint preset.
func NewUTLSTransportWithOptions(opts UTLSOptions) http.RoundTripper {
	return &utlsRoundTripper{
		bindAddr:           opts.BindAddr,
		proxy:              opts.Proxy,
		preset:             opts.Preset,
		insecureSkipVerify: opts.InsecureSkipVerify,
	}
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
		switch strings.ToLower(key) {
		case "content-length", "transfer-encoding", "connection", "proxy-connection", "upgrade":
			// Framing is written below from ContentLength; forwarding the
			// client's own framing headers would either duplicate it or
			// promise a body the request does not have.
			continue
		}
		for _, val := range vals {
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(val)
			sb.WriteString("\r\n")
		}
	}

	// Framing. A POST without Content-Length or chunked encoding is not a
	// valid HTTP/1.1 message: the upstream reads the body as the next
	// request line, stalls and eventually resets the connection. The
	// transport previously wrote no framing at all, so every request with a
	// body (all chat completions) was refused by the upstream.
	chunked := false
	hasBody := req.Body != nil && req.Body != http.NoBody && req.ContentLength != 0
	if hasBody {
		if req.ContentLength > 0 {
			sb.WriteString("Content-Length: ")
			sb.WriteString(strconv.FormatInt(req.ContentLength, 10))
			sb.WriteString("\r\n")
		} else {
			sb.WriteString("Transfer-Encoding: chunked\r\n")
			chunked = true
		}
	}
	sb.WriteString("\r\n")

	if _, err := conn.Write([]byte(sb.String())); err != nil {
		return err
	}
	if !hasBody {
		return nil
	}
	return writeHTTPBody(conn, req.Body, chunked)
}

// writeHTTPBody copies the request body to the wire, framing it as chunks when
// the length was not known up front.
func writeHTTPBody(conn net.Conn, body io.Reader, chunked bool) error {
	if !chunked {
		_, err := io.Copy(conn, body)
		return err
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := io.WriteString(conn, strconv.FormatInt(int64(n), 16)+"\r\n"); werr != nil {
				return werr
			}
			if _, werr := conn.Write(buf[:n]); werr != nil {
				return werr
			}
			if _, werr := io.WriteString(conn, "\r\n"); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			_, werr := io.WriteString(conn, "0\r\n\r\n")
			return werr
		}
		if err != nil {
			return err
		}
	}
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
