package sessionhub

import (
	"net/http"
	"testing"
)

func TestPoolMembers_AppliesHeadersToMembers(t *testing.T) {
	hubCfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"public-tier": {
				Enabled: true,
				Headers: []HeaderRule{
					{Name: "x-session-ext", Mode: HeaderModeMapOrGenerate, Prefix: "ses_", Length: 26, Charset: "hex"},
					{Name: "x-session-client", Mode: HeaderModeStatic, Value: "cli"},
				},
			},
		},
	}
	h := New(hubCfg)
	h.SetPoolMembership("public-tier", []string{
		"vllm-ft-main", "vllm-ft-backup", "vllm-ft-backup-2",
		"vllm-ft-backup-3", "vllm-ft-backup-4", "vllm-ft-backup-5",
	})

	// Apply for a pool member
	headers := http.Header{}
	result := h.Apply(headers, "vllm-ft-main")
	if result == nil {
		t.Fatal("Apply returned nil for pool member vllm-ft-main — pool rule not expanded")
	}
	if headers.Get("X-Session-Client") != "cli" {
		t.Errorf("x-session-client = %q, want cli", headers.Get("X-Session-Client"))
	}
	if got := headers.Get("X-Session-Ext"); got == "" {
		t.Error("x-session-ext is empty")
	}

	// Apply for the pool itself
	headers2 := http.Header{}
	result2 := h.Apply(headers2, "public-tier")
	if result2 == nil {
		t.Fatal("Apply returned nil for pool name public-tier")
	}

	// Apply for an unrelated provider
	headers3 := http.Header{}
	result3 := h.Apply(headers3, "unrelated")
	if result3 != nil {
		t.Errorf("Apply returned non-nil for unrelated provider: %v", result3)
	}
}
