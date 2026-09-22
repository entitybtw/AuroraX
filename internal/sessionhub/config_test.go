package sessionhub

import "testing"

func TestEnsureupstreamRules_AddsMissingHeaders(t *testing.T) {
	// A fresh, empty rule should receive all four canonical upstream headers.
	merged, added, already := EnsureupstreamRules(ProviderRule{})

	if len(added) != 4 {
		t.Fatalf("added = %v, want all four upstream headers", added)
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

func TestEnsureupstreamRules_IsIdempotent(t *testing.T) {
	// Running ensure twice must not duplicate headers or add anything new.
	first, _, _ := EnsureupstreamRules(ProviderRule{})
	second, added, already := EnsureupstreamRules(first)

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

func TestEnsureupstreamRules_PreservesCustomValues(t *testing.T) {
	// An operator who customised x-opencode-project must keep that value.
	custom := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-opencode-project", Mode: HeaderModeStatic, Value: "my-team"},
		},
	}

	merged, added, already := EnsureupstreamRules(custom)

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
	canonical, _, _ := EnsureupstreamRules(ProviderRule{})
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
	// Missing headers are not "non-default" — EnsureupstreamRules adds them.
	// Only present-but-changed headers should be reported.
	diff := HeaderRuleDiff(ProviderRule{})
	if len(diff) != 0 {
		t.Fatalf("diff = %v, want none for an empty rule", diff)
	}
}
