package sessionhub

import "testing"

// sessionRule builds the four session headers the emulated client sends in the
// given mode, all sharing one template value.
func sessionRule(mode HeaderMode) []HeaderRule {
	names := []string{"x-opencode-session", "x-opencode-session-id", "x-session-affinity", "x-session-id"}
	rules := make([]HeaderRule, 0, len(names))
	for _, name := range names {
		rules = append(rules, HeaderRule{
			Name: name, Mode: mode, Prefix: "ses_", Length: 26,
			Charset: CharsetTimestamped, Template: "session",
		})
	}
	return rules
}

// TestAlignRules_UpgradesMapOrGenerateToGenerate is the upgrade path that
// matters in practice: an install created when the extension still mapped
// sessions keeps mapping forever unless apply aligns it with the documented
// per-request behaviour.
func TestAlignRules_UpgradesMapOrGenerateToGenerate(t *testing.T) {
	existing := ProviderRule{Enabled: true, Headers: sessionRule(HeaderModeMapOrGenerate)}

	merged, added, kept, aligned := AlignRules(existing, sessionRule(HeaderModeGenerate))

	if len(added) != 0 {
		t.Fatalf("added = %v, want none", added)
	}
	if len(kept) != 0 {
		t.Fatalf("kept = %v, want none (all four were aligned)", kept)
	}
	if len(aligned) != 4 {
		t.Fatalf("aligned = %v, want all four session headers", aligned)
	}
	for _, h := range merged.Headers {
		if h.Mode != HeaderModeGenerate {
			t.Fatalf("%s mode = %q, want generate", h.Name, h.Mode)
		}
		if h.Template != "session" {
			t.Fatalf("%s template = %q, want session", h.Name, h.Template)
		}
		if h.Length != 26 || h.Charset != CharsetTimestamped {
			t.Fatalf("%s lost its shape: %+v", h.Name, h)
		}
	}
}

// TestAlignRules_UpgradesMapToGenerate covers the strict "map" mode, which
// reuses a stored id no matter how many requests arrive.
func TestAlignRules_UpgradesMapToGenerate(t *testing.T) {
	existing := ProviderRule{Enabled: true, Headers: sessionRule(HeaderModeMap)}

	merged, _, _, aligned := AlignRules(existing, sessionRule(HeaderModeGenerate))

	if len(aligned) != 4 {
		t.Fatalf("aligned = %v, want all four", aligned)
	}
	for _, h := range merged.Headers {
		if h.Mode != HeaderModeGenerate {
			t.Fatalf("%s mode = %q", h.Name, h.Mode)
		}
	}
}

// TestAlignRules_KeepsOperatorCustomisation keeps the narrow guarantee: a
// header an operator pinned to a static value is never rewritten, and an
// unknown extra header is left untouched.
func TestAlignRules_KeepsOperatorCustomisation(t *testing.T) {
	existing := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-opencode-client", Mode: HeaderModeStatic, Value: "cli"},
			{Name: "x-demo-project", Mode: HeaderModeStatic, Value: "my-team"},
		},
	}

	merged, added, kept, aligned := AlignRules(existing, []HeaderRule{
		{Name: "x-opencode-client", Mode: HeaderModeStatic, Value: "cli"},
		{Name: "x-opencode-session", Mode: HeaderModeGenerate, Prefix: "ses_", Length: 26, Charset: CharsetTimestamped, Template: "session"},
	})

	if len(aligned) != 0 {
		t.Fatalf("aligned = %v, want none", aligned)
	}
	if len(kept) != 1 || kept[0] != "x-opencode-client" {
		t.Fatalf("kept = %v, want the static client header", kept)
	}
	if len(added) != 1 || added[0] != "x-opencode-session" {
		t.Fatalf("added = %v, want the missing session header", added)
	}
	byName := map[string]HeaderRule{}
	for _, h := range merged.Headers {
		byName[h.Name] = h
	}
	if got := byName["x-opencode-client"]; got.Mode != HeaderModeStatic || got.Value != "cli" {
		t.Fatalf("static header was rewritten: %+v", got)
	}
	if got := byName["x-demo-project"]; got.Value != "my-team" {
		t.Fatalf("unrelated header was touched: %+v", got)
	}
}

// TestAlignRules_DoesNotDowngradeGenerate makes sure a header that is already
// per-request is not turned back into a mapping by a stale extension payload.
func TestAlignRules_DoesNotDowngradeGenerate(t *testing.T) {
	existing := ProviderRule{Enabled: true, Headers: sessionRule(HeaderModeGenerate)}

	merged, _, kept, aligned := AlignRules(existing, sessionRule(HeaderModeMapOrGenerate))

	if len(aligned) != 0 {
		t.Fatalf("aligned = %v, want none", aligned)
	}
	if len(kept) != 4 {
		t.Fatalf("kept = %v, want all four", kept)
	}
	for _, h := range merged.Headers {
		if h.Mode != HeaderModeGenerate {
			t.Fatalf("%s downgraded to %q", h.Name, h.Mode)
		}
	}
}

// TestTransform_GenerateIsPerRequest is the behaviour the upgrade exists for:
// two requests must not share a session id once the mode is generate.
func TestTransform_GenerateIsPerRequest(t *testing.T) {
	rule := ProviderRule{Enabled: true, Headers: sessionRule(HeaderModeGenerate)}
	store := NewStore()

	first := Transform(make(map[string][]string), "p", rule, store)
	second := Transform(make(map[string][]string), "p", rule, store)

	if first["X-Opencode-Session"] == "" {
		t.Fatal("no session id generated")
	}
	if first["X-Opencode-Session"] != first["X-Session-Id"] {
		t.Fatalf("session headers disagree: %v", first)
	}
	if first["X-Opencode-Session"] == second["X-Opencode-Session"] {
		t.Fatalf("same session id on two requests: %q", first["X-Opencode-Session"])
	}
}
