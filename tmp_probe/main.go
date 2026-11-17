// Temporary fingerprint probe (not part of the shipping gateway).
//
// Reads one JSON request per line from stdin and prints one JSON answer per
// line, so a caller can run many controlled requests through the same uTLS
// transport the gateway uses. It exists to find out *which* part of the
// emulated client the upstream's free-tier check is anchored to: identity
// headers, session shape, tool schema, body shape or TLS. Every field is
// overridable per request.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	httpclient "aurora/internal/http_client"
)

type probeRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Model   string            `json:"model"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Preset  string            `json:"preset"`
	BindIP  string            `json:"bind_ip"`
	Proxy   string            `json:"proxy"`
}

type probeAnswer struct {
	Status  int    `json:"status"`
	Elapsed int64  `json:"elapsed_ms"`
	Preset  string `json:"preset"`
	Error   string `json:"error"`
	Body    string `json:"body"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 8<<20)
	out := json.NewEncoder(os.Stdout)

	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var req probeRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = out.Encode(probeAnswer{Error: "bad request: " + err.Error()})
			continue
		}
		_ = out.Encode(probe(req))
	}
}

func probe(p probeRequest) probeAnswer {
	ans := probeAnswer{Preset: p.Preset}

	opts := httpclient.UTLSOptions{BindAddr: p.BindIP, Preset: p.Preset}
	if p.Proxy != "" {
		u, err := url.Parse(p.Proxy)
		if err != nil || u.Host == "" {
			ans.Error = "bad proxy"
			return ans
		}
		opts.Proxy = u
	}
	rt := httpclient.NewUTLSTransportWithOptions(opts)

	method := p.Method
	if method == "" {
		method = http.MethodPost
	}
	body := p.Body
	if body == "" {
		body = fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"max_tokens":8,"stream":false}`, p.Model)
	}

	req, err := http.NewRequest(method, p.URL, strings.NewReader(body))
	if err != nil {
		ans.Error = err.Error()
		return ans
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := rt.RoundTrip(req)
	ans.Elapsed = time.Since(start).Milliseconds()
	if err != nil {
		ans.Error = err.Error()
		return ans
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	ans.Status = resp.StatusCode
	ans.Body = strings.TrimSpace(string(raw))
	return ans
}
