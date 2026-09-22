package sessionhub

import (
	"regexp"
	"testing"
)

func TestEnsureOpenCodeRules_AddsMissingHeaders(t *testing.T) {
	// A fresh, empty rule should receive all four canonical OpenCode headers.
	merged, added, already := EnsureOpenCodeRules(ProviderRule{})

	if len(added) != 4 {
		t.Fatalf("added = %v, want all four OpenCode headers", added)
	}
	if len(already) != 0 {
		t.Fatalf("already = %v, want none", already)
	}
	if !merged.Enabled {
		t.Fatal("expected merged rule to be enabled")
	}
	if len(merged.Headers) != 4 {
		t.Fatalf("headers = %d, want 4", len(merged.Headers))
	}
}

func TestEnsureOpenCodeRules_IsIdempotent(t *testing.T) {
	// Running ensure twice must not duplicate headers or add anything new.
	first, _, _ := EnsureOpenCodeRules(ProviderRule{})
	second, added, already := EnsureOpenCodeRules(first)

	if len(added) != 0 {
		t.Fatalf("second run added = %v, want none", added)
	}
	if len(already) != 4 {
		t.Fatalf("second run already = %v, want all four", already)
	}
	if len(second.Headers) != 4 {
		t.Fatalf("second run headers = %d, want 4 (no duplicates)", len(second.Headers))
	}
}

func TestEnsureOpenCodeRules_PreservesCustomValues(t *testing.T) {
	// An operator who customised x-opencode-project must keep that value.
	custom := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-opencode-project", Mode: HeaderModeStatic, Value: "my-team"},
		},
	}

	merged, added, already := EnsureOpenCodeRules(custom)

	if len(already) != 1 || already[0] != "x-opencode-project" {
		t.Fatalf("already = %v, want [x-opencode-project]", already)
	}
	if len(added) != 3 {
		t.Fatalf("added = %v, want the three missing headers", added)
	}

	var project *HeaderRule
	for i := range merged.Headers {
		if merged.Headers[i].Name == "x-opencode-project" {
			project = &merged.Headers[i]
		}
	}
	if project == nil || project.Value != "my-team" {
		t.Fatalf("custom project value was overwritten: %+v", project)
	}
}

func TestHeaderRuleDiff_ReportsNonDefault(t *testing.T) {
	// Canonical defaults produce no diff.
	canonical, _, _ := EnsureOpenCodeRules(ProviderRule{})
	if diff := HeaderRuleDiff(canonical); len(diff) != 0 {
		t.Fatalf("diff = %v, want none for canonical rules", diff)
	}

	// A changed project value must be reported.
	custom := ProviderRule{
		Headers: []HeaderRule{
			{Name: "x-opencode-project", Mode: HeaderModeStatic, Value: "changed"},
		},
	}
	diff := HeaderRuleDiff(custom)
	if len(diff) != 1 || diff[0].Name != "x-opencode-project" {
		t.Fatalf("diff = %v, want [x-opencode-project]", diff)
	}
}

func TestHeaderRuleDiff_IgnoresMissing(t *testing.T) {
	// Missing headers are not "non-default" — EnsureOpenCodeRules adds them.
	// Only present-but-changed headers should be reported.
	diff := HeaderRuleDiff(ProviderRule{})
	if len(diff) != 0 {
		t.Fatalf("diff = %v, want none for an empty rule", diff)
	}
}

func TestOpenCodeHeaderRules_ZenSafeShape(t *testing.T) {
	// The free tier only accepts ses_+26 hex and msg_+26 hex. Guard the
	// canonical rule shape so a future edit cannot silently break free tier.
	byName := map[string]HeaderRule{}
	for _, h := range OpenCodeHeaderRules() {
		byName[h.Name] = h
	}
	session := byName["x-opencode-session"]
	if session.Prefix != "ses_" || session.Length != 26 || NormalizeCharset(session.Charset) != CharsetHex {
		t.Fatalf("session rule not zen-safe: %+v", session)
	}
	request := byName["x-opencode-request"]
	if request.Prefix != "msg_" || request.Length != 26 || NormalizeCharset(request.Charset) != CharsetHex {
		t.Fatalf("request rule not zen-safe: %+v", request)
	}
}

func TestGenerateID_Charsets(t *testing.T) {
	hex := GenerateID(26, CharsetHex)
	if len(hex) != 26 {
		t.Fatalf("hex length = %d, want 26", len(hex))
	}
	if !regexp.MustCompile(`^[0-9a-f]{26}$`).MatchString(hex) {
		t.Fatalf("hex charset contains non-hex: %q", hex)
	}

	digits := GenerateID(10, CharsetDigits)
	if !regexp.MustCompile(`^[0-9]{10}$`).MatchString(digits) {
		t.Fatalf("digits charset invalid: %q", digits)
	}

	// Empty charset defaults to alphanumeric (backwards compatible).
	alpha := GenerateID(40, "")
	if len(alpha) != 40 {
		t.Fatalf("alnum length = %d, want 40", len(alpha))
	}
}

func TestGenerateValue_PrefixAndLength(t *testing.T) {
	v := GenerateValue("ses_", 26, CharsetHex)
	if len(v) != 30 || v[:4] != "ses_" {
		t.Fatalf("value = %q, want ses_ + 26 hex", v)
	}
}
