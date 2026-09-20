package httpclient

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
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
	deadline := time.Now().Add(15 * time.Second)
	uconn.SetDeadline(deadline)

	h2 := &h2Conn{uconn: uconn}

	if _, err := uconn.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")); err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls h2: preface: %w", err)
	}

	h2.sendFrame(0, typeSettings, flagNone, h2EncodeSettings([][2]uint32{
		{1, 65536},
		{3, 1000},
		{4, 6291456},
		{5, 16384},
		{6, 262144},
	}))

	if err := h2.readUntilSettingsAck(); err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls h2: settings handshake: %w", err)
	}

	h2.sendFrame(0, typeWindowUpdate, flagNone, []byte{0, 0, 64, 0})

	uconn.SetDeadline(time.Time{})

	resp, err := h2.sendRequest(req)
	if err != nil {
		uconn.Close()
		return nil, fmt.Errorf("utls h2: send request: %w", err)
	}

	resp.Body = &h2Body{h2: h2, resp: resp}
	return resp, nil
}

const (
	typeData         uint8 = 0
	typeHeaders      uint8 = 1
	typeSettings     uint8 = 4
	typeWindowUpdate uint8 = 8
	typeGoAway       uint8 = 7
	typePing         uint8 = 6
	flagNone         uint8 = 0
	flagEndStream    uint8 = 0x1
	flagACK          uint8 = 0x1
	flagEndHeaders   uint8 = 0x4
)

type h2Conn struct {
	uconn   *utls.UConn
	readBuf []byte
}

func (h *h2Conn) sendFrame(streamID uint32, frameType uint8, flags uint8, payload []byte) {
	length := len(payload)
	frame := make([]byte, 9+length)
	frame[0] = byte(length >> 16)
	frame[1] = byte(length >> 8)
	frame[2] = byte(length)
	frame[3] = frameType
	frame[4] = flags
	binary.BigEndian.PutUint32(frame[5:9], streamID)
	copy(frame[9:], payload)
	h.uconn.Write(frame)
}

func (h *h2Conn) readFrame() (uint32, uint8, uint8, []byte, error) {
	for {
		for len(h.readBuf) >= 9 {
			length := int(h.readBuf[0])<<16 | int(h.readBuf[1])<<8 | int(h.readBuf[2])
			frameType := h.readBuf[3]
			flags := h.readBuf[4]
			streamID := binary.BigEndian.Uint32(h.readBuf[5:9]) & 0x7fffffff

			if len(h.readBuf) < 9+length {
				break
			}

			payload := make([]byte, length)
			copy(payload, h.readBuf[9:9+length])
			h.readBuf = h.readBuf[9+length:]

			return streamID, frameType, flags, payload, nil
		}

		buf := make([]byte, 65536)
		n, err := h.uconn.Read(buf)
		if err != nil {
			return 0, 0, 0, nil, err
		}
		h.readBuf = append(h.readBuf, buf[:n]...)
	}
}

func (h *h2Conn) readUntilSettingsAck() error {
	for {
		sid, ftype, flags, payload, err := h.readFrame()
		if err != nil {
			return fmt.Errorf("read frame: %w", err)
		}

		switch {
		case ftype == typeGoAway:
			return fmt.Errorf("GOAWAY on stream %d: %s", sid, string(payload))
		case ftype == typePing && flags&flagACK == 0:
			h.sendFrame(0, typePing, flagACK, payload)
		case ftype == typeSettings && flags&flagACK != 0:
			return nil
		case ftype == typeSettings:
			h.sendFrame(0, typeSettings, flagACK, nil)
		}
	}
}

func (h *h2Conn) sendRequest(req *http.Request) (*http.Response, error) {
	sid := uint32(1)

	var hdrs []byte
	hdrs = append(hdrs, hpackIndexedMethod(req.Method)...)
	hdrs = appendHPACKString(hdrs, ":path", req.URL.RequestURI())
	hdrs = appendHPACKString(hdrs, ":scheme", "https")
	hdrs = appendHPACKString(hdrs, ":authority", req.URL.Host)

	for key, vals := range req.Header {
		for _, val := range vals {
			hdrs = appendHPACKString(hdrs, key, val)
		}
	}

	if req.Body != nil && req.ContentLength > 0 {
		hdrs = appendHPACKString(hdrs, "content-length", strconv.FormatInt(req.ContentLength, 10))
	}

	flags := flagEndHeaders | flagEndStream
	h.sendFrame(sid, typeHeaders, flags, hdrs)

	if req.Body != nil {
		defer req.Body.Close()
		buf := make([]byte, 16384)
		for {
			n, readErr := req.Body.Read(buf)
			if n > 0 {
				df := flagNone
				if readErr != nil {
					df = flagEndStream
				}
				h.sendFrame(sid, typeData, df, buf[:n])
			}
			if readErr != nil {
				break
			}
		}
	}

	return h.readResponse(sid)
}

func (h *h2Conn) readResponse(streamID uint32) (*http.Response, error) {
	headers := make(map[string]string)
	var dataBuf []byte

	for {
		sid, ftype, flags, payload, err := h.readFrame()
		if err != nil {
			return nil, err
		}

		if ftype == typeGoAway {
			return nil, fmt.Errorf("h2: GOAWAY received")
		}

		if ftype == typePing && flags&flagACK == 0 {
			h.sendFrame(0, typePing, flagACK, payload)
			continue
		}

		if sid == 0 {
			switch {
			case ftype == typeSettings && flags&flagACK != 0:
			case ftype == typeSettings:
				h.sendFrame(0, typeSettings, flagACK, nil)
			}
			continue
		}

		if sid != streamID {
			continue
		}

		switch ftype {
		case typeHeaders:
			decoded, err := hpackDecode(payload)
			if err != nil {
				return nil, fmt.Errorf("h2: hpack decode: %w", err)
			}
			for k, v := range decoded {
				headers[k] = v
			}
		case typeData:
			dataBuf = append(dataBuf, payload...)
		}

		if flags&flagEndStream != 0 {
			break
		}
	}

	statusCode := 200
	if s := headers[":status"]; s != "" {
		statusCode, _ = strconv.Atoi(s)
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Proto:      "HTTP/2.0",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     make(http.Header),
	}

	for k, v := range headers {
		if k == ":status" || k == ":path" {
			continue
		}
		resp.Header.Set(k, v)
	}

	resp.Body = io.NopCloser(bytes.NewReader(dataBuf))
	return resp, nil
}

type h2Body struct {
	h2   *h2Conn
	resp *http.Response
	done bool
}

func (b *h2Body) Read(p []byte) (int, error) {
	if b.done {
		return 0, io.EOF
	}
	n, err := b.resp.Body.Read(p)
	if err != nil {
		b.done = true
		return n, io.EOF
	}
	return n, nil
}

func (b *h2Body) Close() error {
	b.done = true
	b.h2.uconn.Close()
	return nil
}

func h2EncodeSettings(settings [][2]uint32) []byte {
	var payload []byte
	for _, s := range settings {
		payload = append(payload, byte(s[0]>>8), byte(s[0]))
		payload = append(payload, byte(s[1]>>24), byte(s[1]>>16), byte(s[1]>>8), byte(s[1]))
	}
	return payload
}

func hpackIndexedMethod(method string) []byte {
	switch method {
	case "GET":
		return []byte{0x82}
	case "POST":
		return []byte{0x83}
	case "PUT":
		return []byte{0x84}
	case "DELETE":
		return []byte{0x85}
	case "HEAD":
		return []byte{0x86}
	default:
		return appendHPACKString(nil, ":method", method)
	}
}

func appendHPACKString(buf []byte, name, value string) []byte {
	buf = append(buf, 0x40)
	buf = appendHPACKRawString(buf, name)
	buf = appendHPACKRawString(buf, value)
	return buf
}

func appendHPACKRawString(buf []byte, s string) []byte {
	n := len(s)
	if n < 128 {
		buf = append(buf, byte(n))
	} else {
		buf = append(buf, byte(128|n&0x7f))
		n >>= 7
		for n > 0 {
			buf = append(buf, byte(n&0x7f))
			n >>= 7
		}
	}
	buf = append(buf, s...)
	return buf
}

func hpackDecode(payload []byte) (map[string]string, error) {
	headers := make(map[string]string)
	off := 0

	for off < len(payload) {
		b := payload[off]

		if b&0x80 != 0 {
			idx := int(b & 0x7f)
			off++
			if idx < 128 {
				name, value := hpackStaticTableLookup(idx)
				if name != "" {
					headers[name] = value
				}
			}
			continue
		}

		if b&0xc0 == 0x40 {
			off++
			nameLen, n := hpackReadInt(6, payload, off)
			off = n
			name := string(payload[off : off+nameLen])
			off += nameLen
			valLen, n := hpackReadInt(7, payload, off)
			off = n
			value := string(payload[off : off+valLen])
			off += valLen
			headers[name] = value
			continue
		}

		if b&0xe0 == 0x20 {
			off++
			nameLen, n := hpackReadInt(5, payload, off)
			off = n
			name := string(payload[off : off+nameLen])
			off += nameLen
			valLen, n := hpackReadInt(7, payload, off)
			off = n
			value := string(payload[off : off+valLen])
			off += valLen
			headers[name] = value
			continue
		}

		if b&0xf0 == 0x10 {
			off++
			nameLen, n := hpackReadInt(4, payload, off)
			off = n
			name := string(payload[off : off+nameLen])
			off += nameLen
			valLen, n := hpackReadInt(7, payload, off)
			off = n
			value := string(payload[off : off+valLen])
			off += valLen
			headers[name] = value
			continue
		}

		off++
	}

	return headers, nil
}

func hpackReadInt(prefixBits int, payload []byte, off int) (int, int) {
	if off >= len(payload) {
		return 0, off
	}
	maxPrefix := (1 << uint(prefixBits)) - 1
	val := int(payload[off] & byte(maxPrefix))
	off++
	if val < maxPrefix {
		return val, off
	}
	m := 0
	for off < len(payload) {
		b := payload[off]
		off++
		val += int(b&0x7f) << uint(m)
		m += 7
		if b&0x80 == 0 {
			break
		}
	}
	return val, off
}

func hpackStaticTableLookup(idx int) (string, string) {
	table := [][2]string{
		{":authority", ""},
		{":method", "GET"},
		{":method", "POST"},
		{":path", "/"},
		{":path", "/index.html"},
		{":scheme", "http"},
		{":scheme", "https"},
		{":status", "200"},
		{":status", "204"},
		{":status", "304"},
		{"accept-charset", ""},
		{"accept-encoding", "gzip, deflate"},
		{"accept-language", ""},
		{"accept-ranges", ""},
		{"accept", ""},
		{"access-control-allow-origin", ""},
		{"age", ""},
		{"allow", ""},
		{"authorization", ""},
		{"content-disposition", ""},
		{"content-encoding", ""},
		{"content-language", ""},
		{"content-length", ""},
		{"content-location", ""},
		{"content-range", ""},
		{"content-type", ""},
		{"cookie", ""},
		{"date", ""},
		{"etag", ""},
		{"expect", ""},
		{"expires", ""},
		{"from", ""},
		{"host", ""},
		{"if-match", ""},
		{"if-modified-since", ""},
		{"if-none-match", ""},
		{"if-range", ""},
		{"if-unmodified-since", ""},
		{"last-modified", ""},
		{"link", ""},
		{"location", ""},
		{"max-forwards", ""},
		{"proxy-authenticate", ""},
		{"proxy-authorization", ""},
		{"range", ""},
		{"referer", ""},
		{"refresh", ""},
		{"retry-after", ""},
		{"server", ""},
		{"set-cookie", ""},
		{"strict-transport-security", ""},
		{"transfer-encoding", ""},
		{"user-agent", ""},
		{"vary", ""},
		{"via", ""},
		{"www-authenticate", ""},
	}

	idx--
	if idx >= 0 && idx < len(table) {
		return table[idx][0], table[idx][1]
	}
	return "", ""
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
