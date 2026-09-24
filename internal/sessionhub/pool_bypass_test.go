package sessionhub

import (
	"net/http"
	"testing"
)

func TestPoolBypass_AppliesHeadersToMembers(t *testing.T) {
	hubCfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"opencode-zen": {
				Enabled: true,
				Headers: []HeaderRule{
					{Name: "x-opencode-session", Mode: HeaderModeMapOrGenerate, Prefix: "ses_", Length: 26, Charset: "hex"},
					{Name: "x-opencode-client", Mode: HeaderModeStatic, Value: "cli"},
				},
			},
		},
	}
	h := New(hubCfg)
	h.SetPoolMembership("opencode-zen", []string{
		"vllm-zen-main", "vllm-zen-backup", "vllm-zen-backup-2",
		"vllm-zen-backup-3", "vllm-zen-backup-4", "vllm-zen-backup-5",
	})

	// Apply for a pool member
	headers := http.Header{}
	result := h.Apply(headers, "vllm-zen-main")
	if result == nil {
		t.Fatal("Apply returned nil for pool member vllm-zen-main — pool rule not expanded")
	}
	if headers.Get("X-Opencode-Client") != "cli" {
		t.Errorf("x-opencode-client = %q, want cli", headers.Get("X-Opencode-Client"))
	}
	if got := headers.Get("X-Opencode-Session"); got == "" {
		t.Error("x-opencode-session is empty")
	}

	// Apply for the pool itself
	headers2 := http.Header{}
	result2 := h.Apply(headers2, "opencode-zen")
	if result2 == nil {
		t.Fatal("Apply returned nil for pool name opencode-zen")
	}

	// Apply for an unrelated provider
	headers3 := http.Header{}
	result3 := h.Apply(headers3, "unrelated")
	if result3 != nil {
		t.Errorf("Apply returned non-nil for unrelated provider: %v", result3)
	}
}
