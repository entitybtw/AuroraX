package addon

import (
	"testing"
)

// TestYaegiSupportsAddonCapabilities pins the stdlib surface the shipped
// extensions rely on. Yaegi links its own symbol table, which trails the
// installed Go release — if a future upgrade drops one of these, the addons
// fail at load time with an unhelpful message. Failing here first names the
// missing capability.
func TestYaegiSupportsAddonCapabilities(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			"goroutine and mutex",
			`package main

import "sync"

var mu sync.Mutex
var done = make(chan bool)

func Start() string {
	go func() {
		mu.Lock()
		mu.Unlock()
		done <- true
	}()
	<-done
	return "ok"
}
`,
		},
		{
			"dial with timeout",
			`package main

import "net"

func Probe(addr string) string {
	conn, err := net.DialTimeout("tcp", addr, 10)
	if err != nil {
		return "down"
	}
	conn.Close()
	return "up"
}
`,
		},
		{
			"http client",
			`package main

import "net/http"

func Fetch(url string) string {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return err.Error()
	}
	resp.Body.Close()
	return resp.Status
}
`,
		},
		{
			"base64 and json",
			`package main

import (
	"encoding/base64"
	"encoding/json"
)

func Decode(s string) string {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return string(raw)
}
`,
		},
		{
			"state file round trip",
			`package main

import (
	"os"
	"path/filepath"
)

func Write(dir string) string {
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAddon(t, dir, "capability-check.go", "// addon-kind: runtime\n"+tc.src)
			s := NewStore(dir)
			s.Load()
			if errs := s.Status().Errors; len(errs) > 0 {
				t.Fatalf("Yaegi cannot compile %s: %+v", tc.name, errs)
			}
			if len(s.List()) != 1 {
				t.Fatalf("addon did not load: %+v", s.Status())
			}
		})
	}
}
