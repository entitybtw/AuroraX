package sessionhub

import (
	"net/http"
	"testing"
)

func BenchmarkApplyHit(b *testing.B) {
	cfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"opencode-zen": {Enabled: true, Headers: []HeaderRule{{
				Name: "x-opencode-session", Mode: HeaderModeMap, Prefix: "ses_", Length: 28,
			}}},
		},
	}
	h := New(cfg)
	h.SetPoolMembership("opencode-zen", []string{"acc1", "acc2", "acc3"})
	inbound := "ses_hello"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hd := http.Header{}
		hd.Set("x-opencode-session", inbound)
		h.Apply(hd, "acc1")
	}
}

func BenchmarkApplyMiss(b *testing.B) {
	h := New(&HubConfig{Enabled: true, Providers: map[string]ProviderRule{}})
	inbound := "ses_hello"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hd := http.Header{}
		hd.Set("x-opencode-session", inbound)
		h.Apply(hd, "nope")
	}
}
