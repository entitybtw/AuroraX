package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// The charset of a session hub header rule selects the alphabet for generated
// identity values. The "timestamped" charset is the upstream's own id factory,
// whose ids embed the current time: upstreams that gate free-tier access
// decode that head back to a clock and reject ids that do not resolve to a
// recent time.
//
// The field has to survive a YAML round trip. It used to be dropped on the way
// from the persisted rules file into the session hub, so every rule silently
// degraded to a plain random id after a gateway restart — the symptom being
// an upstream that answers 403 FreeTierError for a request the very same
// configuration had been accepted for before the restart.
func TestSessionHubHeaderRuleKeepsCharsetAcrossYAML(t *testing.T) {
	const doc = `
enabled: true
providers:
    opencode-zen:
        enabled: true
        headers:
            - name: x-opencode-session
              mode: map_or_generate
              prefix: ses_
              length: 26
              charset: timestamped
            - name: x-opencode-client
              mode: static
              value: cli
`
	var cfg SessionHubConfig
	if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rule, ok := cfg.Providers["opencode-zen"]
	if !ok {
		t.Fatal("provider rule missing")
	}
	if len(rule.Headers) != 2 {
		t.Fatalf("headers = %d, want 2", len(rule.Headers))
	}
	if got := rule.Headers[0].Charset; got != "timestamped" {
		t.Fatalf("charset = %q, want %q", got, "timestamped")
	}
	if got := rule.Headers[1].Charset; got != "" {
		t.Fatalf("charset of a rule without one = %q, want empty", got)
	}
}
