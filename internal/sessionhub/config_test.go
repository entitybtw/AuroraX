package sessionhub

import (
	"regexp"
	"testing"
)

func testHeaderSet() []HeaderRule {
	return []HeaderRule{
		{Name: "x-demo-session", Mode: HeaderModeMapOrGenerate, Prefix: "ses_", Length: 26, Charset: CharsetHex},
		{Name: "x-demo-client", Mode: HeaderModeStatic, Value: "cli"},
		{Name: "x-demo-request", Mode: HeaderModeGenerate, Prefix: "msg_", Length: 26, Charset: CharsetHex},
		{Name: "x-demo-project", Mode: HeaderModeStatic, Value: "global"},
	}
}

func TestEnsureRules_AddsMissingHeaders(t *testing.T) {
	merged, added, already := EnsureRules(ProviderRule{}, testHeaderSet())

	if len(added) != 4 {
		t.Fatalf("added = %v, want all four headers", added)
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

func TestEnsureRules_PreservesCustomValues(t *testing.T) {
	custom := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-demo-project", Mode: HeaderModeStatic, Value: "my-team"},
		},
	}

	merged, added, already := EnsureRules(custom, testHeaderSet())

	if len(already) != 1 || already[0] != "x-demo-project" {
		t.Fatalf("already = %v, want [x-demo-project]", already)
	}
	if len(added) != 3 {
		t.Fatalf("added = %v, want the three missing headers", added)
	}

	var project *HeaderRule
	for i := range merged.Headers {
		if merged.Headers[i].Name == "x-demo-project" {
			project = &merged.Headers[i]
		}
	}
	if project == nil || project.Value != "my-team" {
		t.Fatalf("custom project value was overwritten: %+v", project)
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

func TestDefaultHeaderRules_Empty(t *testing.T) {
	if got := defaultHeaderRules(); len(got) != 0 {
		t.Fatalf("defaultHeaderRules = %v, want empty (extensions supply headers)", got)
	}
}
