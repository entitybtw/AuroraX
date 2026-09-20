package httpclient

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

type utlsRoundTripper struct {
	bindAddr string
}

func (rt *utlsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(host, port)

	var localAddr *net.TCPAddr
	if rt.bindAddr != "" {
		localAddr = &net.TCPAddr{IP: net.ParseIP(rt.bindAddr)}
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		LocalAddr: localAddr,
	}

	conn, err := dialer.DialContext(req.Context(), "tcp", addr)
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
