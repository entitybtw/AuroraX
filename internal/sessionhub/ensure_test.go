package sessionhub

import "testing"

func TestEnsureRules_AddsAndPreserves(t *testing.T) {
	existing := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "custom", Mode: HeaderModeStatic, Value: "mine"},
		},
	}
	want := []HeaderRule{
		{Name: "custom", Mode: HeaderModeStatic, Value: "ignored"},
		{Name: "x-new", Mode: HeaderModeGenerate, Prefix: "new_", Length: 10},
	}
	merged, added, kept := EnsureRules(existing, want)

	if len(kept) != 1 || kept[0] != "custom" {
		t.Fatalf("kept = %v, want [custom]", kept)
	}
	if len(added) != 1 || added[0] != "x-new" {
		t.Fatalf("added = %v, want [x-new]", added)
	}
	var custom *HeaderRule
	for i := range merged.Headers {
		if merged.Headers[i].Name == "custom" {
			custom = &merged.Headers[i]
		}
	}
	if custom == nil || custom.Value != "mine" {
		t.Fatalf("existing custom rule was overwritten: %+v", custom)
	}
}

func TestEnsureRules_IsIdempotent(t *testing.T) {
	first, _, _ := EnsureRules(ProviderRule{}, []HeaderRule{
		{Name: "a", Mode: HeaderModeGenerate},
		{Name: "b", Mode: HeaderModeStatic, Value: "x"},
	})
	second, added, kept := EnsureRules(first, []HeaderRule{
		{Name: "a", Mode: HeaderModeGenerate},
		{Name: "b", Mode: HeaderModeStatic, Value: "x"},
	})
	if len(added) != 0 || len(kept) != 2 || len(second.Headers) != 2 {
		t.Fatalf("not idempotent: added=%v kept=%v headers=%d", added, kept, len(second.Headers))
	}
}
