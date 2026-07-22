// Package httpclient provides a centralized HTTP client factory with unified configuration.
package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
)

// ClientConfig holds configuration options for creating HTTP clients
type ClientConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host
	MaxIdleConnsPerHost int

	// MaxConnsPerHost optionally limits total active, dialing, and idle connections per host.
	// Zero keeps Go's default unlimited behavior.
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself
	IdleConnTimeout time.Duration

	// Timeout specifies a time limit for requests made by the client
	Timeout time.Duration

	// DialTimeout is the maximum amount of time a dial will wait for a connect to complete
	DialTimeout time.Duration

	// KeepAlive specifies the interval between keep-alive probes for an active network connection
	KeepAlive time.Duration

	// TLSHandshakeTimeout specifies the maximum amount of time to wait for a TLS handshake
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout specifies the amount of time to wait for a server's response headers
	ResponseHeaderTimeout time.Duration

	// HTTPProxy configures an explicit proxy for http:// upstream requests.
	HTTPProxy string

	// HTTPSProxy configures an explicit proxy for https:// upstream requests.
	HTTPSProxy string

	// NoProxy is a comma-separated host list that bypasses configured proxies.
	NoProxy string

	// CACertPEM appends a custom CA certificate for TLS interception proxies.
	CACertPEM string
}

var defaultConfigOverride *ClientConfig

// SetDefaultConfigOverride updates the process-wide default config used by NewDefaultHTTPClient.
func SetDefaultConfigOverride(config *ClientConfig) {
	if config == nil {
		defaultConfigOverride = nil
		return
	}
	copy := *config
	defaultConfigOverride = &copy
}

// globalTransport is a shared http.Transport reused by default clients so that
// all providers share the same connection pool, reducing the number of idle
// connections and file descriptors under high concurrency.
var (
	globalTransport    *http.Transport
	globalTransportMu  sync.Mutex
	globalConfigLoaded bool
)

// ensureGlobalTransport lazily builds (or rebuilds) the global transport from
// the current default configuration. It is safe for concurrent use.
func ensureGlobalTransport() *http.Transport {
	globalTransportMu.Lock()
	defer globalTransportMu.Unlock()
	cfg := defaultClientConfig()
	if globalTransport == nil || !globalConfigLoaded {
		globalTransport = newTransport(cfg)
		globalConfigLoaded = true
	}
	// Rebuild the transport only if the configuration has changed since last
	// build. This intentionally does NOT diff every field — a nil override
	// triggers a rebuild; otherwise we trust the caller to call
	// SetDefaultConfigOverride before any client is constructed.
	return globalTransport
}

func newTransport(config ClientConfig) *http.Transport {
	proxyFunc := proxyFunc(&config)
	tlsConfig := tlsConfig(config.CACertPEM)
	return &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// getEnvDuration reads a duration from an environment variable, returning the default if not set or invalid.
// Accepts either plain integers (interpreted as seconds) or Go duration strings (e.g., "10m", "1h30m").
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	// Try parsing as integer seconds first (simpler for env config)
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	// Fall back to Go duration format (e.g., "10m", "1h30m")
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed < 0 {
		return defaultVal
	}
	return parsed
}

// DefaultConfig returns a ClientConfig with sensible defaults for API clients.
// Timeout values match OpenAI/Anthropic SDK defaults (10 minutes).
// Can be overridden via environment variables (values in seconds, or Go duration format):
//   - HTTP_TIMEOUT: overall request timeout (default: 600)
//   - HTTP_RESPONSE_HEADER_TIMEOUT: time to wait for response headers (default: 600)
//   - HTTP_MAX_IDLE_CONNS: max idle connections across hosts (default: 4096)
//   - HTTP_MAX_IDLE_CONNS_PER_HOST: max idle connections per host (default: 4096)
//   - HTTP_MAX_CONNS_PER_HOST: max active+dialing+idle connections per host (default: 0, unlimited)
//
// Note: These env vars are also documented in config.HTTPConfig. The env var bridge
// here works correctly because the aurora entrypoint loads .env before config and
// providers are initialized.
func DefaultConfig() ClientConfig {
	defaultLongTimeout := 600 * time.Second
	return ClientConfig{
		MaxIdleConns:          getEnvInt("HTTP_MAX_IDLE_CONNS", 4096),
		MaxIdleConnsPerHost:   getEnvInt("HTTP_MAX_IDLE_CONNS_PER_HOST", 4096),
		MaxConnsPerHost:       getEnvInt("HTTP_MAX_CONNS_PER_HOST", 256),
		IdleConnTimeout:       90 * time.Second,
		Timeout:               getEnvDuration("HTTP_TIMEOUT", defaultLongTimeout),
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: getEnvDuration("HTTP_RESPONSE_HEADER_TIMEOUT", defaultLongTimeout),
		HTTPProxy:             os.Getenv("HTTP_PROXY"),
		HTTPSProxy:            os.Getenv("HTTPS_PROXY"),
		NoProxy:               os.Getenv("NO_PROXY"),
	}
}

// NewHTTPClient creates a new HTTP client with the provided configuration.
// If config is nil, DefaultConfig() is used.
// When no custom proxy or TLS configuration is required, the client shares the
// global transport so connection pools are unified across all providers.
func NewHTTPClient(config *ClientConfig) *http.Client {
	if config == nil {
		cfg := defaultClientConfig()
		return &http.Client{
			Transport: ensureGlobalTransport(),
			Timeout:   cfg.Timeout,
		}
	}

	proxyFunc := proxyFunc(config)
	tlsConfig := tlsConfig(config.CACertPEM)

	transport := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
}

// NewDefaultHTTPClient creates a new HTTP client sharing the global transport
// pool with default configuration. All callers that do not need a custom proxy
// or TLS will reuse the same connection pool, reducing idle-connection and file
// descriptor pressure under high concurrency.
func NewDefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: ensureGlobalTransport(),
		Timeout:   DefaultConfig().Timeout,
	}
}

func defaultClientConfig() ClientConfig {
	if defaultConfigOverride != nil {
		return *defaultConfigOverride
	}
	return DefaultConfig()
}

func proxyFunc(config *ClientConfig) func(*http.Request) (*url.URL, error) {
	proxyConfig := httpproxy.Config{
		HTTPProxy:  strings.TrimSpace(config.HTTPProxy),
		HTTPSProxy: strings.TrimSpace(config.HTTPSProxy),
		NoProxy:    strings.TrimSpace(config.NoProxy),
	}
	if proxyConfig.HTTPProxy == "" && proxyConfig.HTTPSProxy == "" && proxyConfig.NoProxy == "" {
		return http.ProxyFromEnvironment
	}
	proxyForURL := proxyConfig.ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		return proxyForURL(req.URL)
	}
}

func tlsConfig(caCertPEM string) *tls.Config {
	caCertPEM = strings.TrimSpace(caCertPEM)
	if caCertPEM == "" {
		return nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM([]byte(caCertPEM)) {
		return nil
	}
	return &tls.Config{RootCAs: pool}
}
