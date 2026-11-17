// Package tlsfront terminates the client side of a request locally and dials
// the real upstream with a chosen TLS fingerprint.
//
// The sidecar that emulates the upstream's own client runs on Bun, and Bun
// offers no way to change its ClientHello. The gateway therefore exposes this
// front on loopback: the sidecar posts plain HTTP to it, the front applies the
// egress exit the gateway picked and opens the upstream connection with uTLS,
// rotating through the configured fingerprint presets request by request.
package tlsfront

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	httpclient "aurora/internal/http_client"
)

// UpstreamHeader carries the real upstream base URL for a request. The sidecar
// keeps its normal base_url and adds this header when it posts to the front,
// so the front never needs to know provider configuration.
const UpstreamHeader = "X-Aurora-Upstream"

// PresetHeader echoes the fingerprint that served the request. It makes
// rotation observable from a single curl.
const PresetHeader = "X-Aurora-TLS-Preset"

// Config controls how the front dials and which fingerprints it rotates.
type Config struct {
	// Presets are ClientHello names (see httpclient.UTLSPresets). Rotation is
	// round-robin in the given order, so the first request uses the first
	// preset. Empty means a single "chrome120" front.
	Presets []string
	// InsecureSkipVerify disables upstream certificate verification. Tests
	// only; never set it in production.
	InsecureSkipVerify bool
}

// owned remembers the listener this process created, for the shutdown path
// in main: a front that was reused (another instance served the port) must not
// be closed by this process, and there is no other handle at that layer.
var (
	ownedMu  sync.Mutex
	ownedFrt *Front
)

// CloseOwned closes the front this process opened, if any. Safe to call more
// than once and from a process that never opened one.
func CloseOwned() {
	ownedMu.Lock()
	front := ownedFrt
	ownedFrt = nil
	ownedMu.Unlock()
	if front != nil {
		_ = front.Close()
	}
}

// Front is a loopback HTTP server that forwards to the upstream with uTLS.
type Front struct {
	presets []string
	cfg     Config
	counter atomic.Uint64
	server  *http.Server
	ln      net.Listener
}

// New validates the preset list and returns a front. Unknown preset names are
// rejected here rather than failing every request later.
func New(cfg Config) (*Front, error) {
	presets := make([]string, 0, len(cfg.Presets))
	for _, raw := range cfg.Presets {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, ok := httpclient.UTLSClientHello(name); !ok {
			return nil, fmt.Errorf("tls front: unknown fingerprint preset %q (known: %s)", raw, strings.Join(httpclient.UTLSPresets(), ", "))
		}
		presets = append(presets, name)
	}
	if len(presets) == 0 {
		presets = []string{"chrome120"}
	}
	return &Front{presets: presets, cfg: cfg}, nil
}

// PresetsFromEnv reads a comma-separated preset list. An empty value yields no
// presets so New applies its default.
func PresetsFromEnv(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// Presets returns the rotation order.
func (f *Front) Presets() []string { return append([]string(nil), f.presets...) }

// Start listens on addr ("127.0.0.1:8091", or ":0" for an ephemeral port) and
// serves until Close. It returns the address actually bound.
func (f *Front) Start(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("tls front: listen %s: %w", addr, err)
	}
	f.ln = ln
	ownedMu.Lock()
	ownedFrt = f
	ownedMu.Unlock()
	f.server = &http.Server{
		Handler:           http.HandlerFunc(f.serveHTTP),
		ReadHeaderTimeout: 15 * time.Second,
	}
	go func() {
		if err := f.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("tls front: serve: %v", err)
		}
	}()
	return ln.Addr().String(), nil
}

// Addr returns the bound address, or "" before Start.
func (f *Front) Addr() string {
	if f.ln == nil {
		return ""
	}
	return f.ln.Addr().String()
}

// StartOrReuse starts the front the way Start does, but treats an occupied
// listen address as "another instance already serves it" instead of a fatal
// error, provided that instance answers like a front.
//
// The address is a fixed loopback port in the container's shared network
// namespace (network_mode: host). A previous container that is still shutting
// down holds it for a few seconds, and a front left behind by a crashed
// gateway holds it forever. Failing startup in both cases turned a routine
// restart into "the whole gateway is down"; reusing a verified front is safe
// because the front is stateless — it applies the headers every request
// carries.
func (f *Front) StartOrReuse(addr string) (string, error) {
	bound, err := f.Start(addr)
	if err == nil {
		return bound, nil
	}
	if !isAddrInUse(err) {
		return "", err
	}
	if !LooksLikeFront(addr) {
		return "", fmt.Errorf("tls front: %w (and the listener is not a front, so it will not be reused)", err)
	}
	log.Printf("tls front: %s is already served by another instance, reusing it", addr)
	return addr, nil
}

// LooksLikeFront reports whether addr answers like a TLS front: a request
// without the upstream header is rejected with 400 and the "tls front" text.
// A stranger on the port therefore never gets mistaken for one.
func LooksLikeFront(addr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "tls front")
}

// isAddrInUse reports whether a listen error is the address being taken.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE) || strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

// Close stops the listener.
func (f *Front) Close() error {
	if f.server == nil {
		return nil
	}
	return f.server.Close()
}

// nextPreset returns the fingerprint for this request.
func (f *Front) nextPreset() string {
	n := f.counter.Add(1) - 1
	return f.presets[int(n%uint64(len(f.presets)))]
}

func (f *Front) serveHTTP(w http.ResponseWriter, r *http.Request) {
	upstream := strings.TrimSpace(r.Header.Get(UpstreamHeader))
	if upstream == "" {
		http.Error(w, "tls front: missing "+UpstreamHeader, http.StatusBadRequest)
		return
	}
	target, err := joinUpstream(upstream, r.URL)
	if err != nil {
		http.Error(w, "tls front: bad upstream: "+err.Error(), http.StatusBadRequest)
		return
	}

	bindAddr := strings.TrimSpace(r.Header.Get("X-Aurora-Bind-Ip"))
	var exitProxy *url.URL
	if raw := strings.TrimSpace(r.Header.Get("X-Aurora-Exit-Proxy")); raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			exitProxy = u
		}
	}

	preset := f.nextPreset()
	rt := httpclient.NewUTLSTransportWithOptions(httpclient.UTLSOptions{
		BindAddr:           bindAddr,
		Proxy:              exitProxy,
		Preset:             preset,
		InsecureSkipVerify: f.cfg.InsecureSkipVerify,
	})

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		http.Error(w, "tls front: build request: "+err.Error(), http.StatusBadRequest)
		return
	}
	outReq.ContentLength = r.ContentLength
	outReq.Header = forwardedHeaders(r.Header)

	resp, err := rt.RoundTrip(outReq)
	if err != nil {
		log.Printf("tls front: %s %s via %s: %v", r.Method, r.URL.Path, preset, err)
		http.Error(w, "tls front: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	w.Header().Set(PresetHeader, preset)
	w.WriteHeader(resp.StatusCode)

	// Stream: a chat completion can be a long SSE body, so every chunk is
	// flushed as it arrives instead of being buffered until the upstream ends.
	rc := http.NewResponseController(w)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			_ = rc.Flush()
		}
		if readErr != nil {
			return
		}
	}
}

// joinUpstream appends the request path and query to the upstream base URL.
func joinUpstream(base string, reqURL *url.URL) (*url.URL, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%q is not an absolute URL", base)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(reqURL.Path, "/")
	u.RawPath = ""
	if reqURL.RawQuery != "" {
		if u.RawQuery != "" {
			u.RawQuery += "&" + reqURL.RawQuery
		} else {
			u.RawQuery = reqURL.RawQuery
		}
	}
	return u, nil
}

// forwardedHeaders copies the client headers that belong to the upstream
// request: everything except hop-by-hop headers and the front's own control
// headers, which must never reach the upstream.
func forwardedHeaders(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, vals := range in {
		canonical := http.CanonicalHeaderKey(key)
		switch canonical {
		case "Host", "Connection", "Keep-Alive", "Proxy-Connection", "Proxy-Authenticate",
			"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
			UpstreamHeader, PresetHeader:
			continue
		}
		if strings.HasPrefix(strings.ToLower(canonical), "x-aurora-") {
			continue
		}
		out[canonical] = append([]string(nil), vals...)
	}
	return out
}

// copyResponseHeaders copies the upstream answer, dropping hop-by-hop headers
// and the upstream's own content length: the body is streamed, so length is
// re-derived (or chunked) by the local server.
func copyResponseHeaders(dst, src http.Header) {
	for key, vals := range src {
		switch http.CanonicalHeaderKey(key) {
		case "Connection", "Keep-Alive", "Proxy-Connection", "Te", "Trailer",
			"Transfer-Encoding", "Upgrade", "Content-Length":
			continue
		}
		dst[http.CanonicalHeaderKey(key)] = append([]string(nil), vals...)
	}
}

// AddrFromEnv returns the listen address from AURORA_TLS_FRONT_LISTEN.
func AddrFromEnv() string {
	return strings.TrimSpace(os.Getenv("AURORA_TLS_FRONT_LISTEN"))
}

// PresetsEnv reads AURORA_TLS_FRONT_PRESETS.
func PresetsEnv() []string {
	return PresetsFromEnv(os.Getenv("AURORA_TLS_FRONT_PRESETS"))
}
