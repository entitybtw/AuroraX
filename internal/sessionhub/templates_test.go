package sessionhub

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// The emulated upstream client sends distributed-tracing headers next to its
// identity headers. Their shape has to be exact — a client that sends a
// malformed trace context looks different from one that does not send one at
// all.
var (
	b3Re = regexp.MustCompile(`^[0-9a-f]{32}-[0-9a-f]{16}-1-[0-9a-f]{16}$`)
	// traceparent: version "-" trace id "-" parent id "-" trace flags.
	traceparentRe = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-01$`)
	// project id: 40 hex characters, the shape of a git object id.
	projectIDRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func TestGenerateB3Shape(t *testing.T) {
	for i := 0; i < 20; i++ {
		v := GenerateB3()
		if !b3Re.MatchString(v) {
			t.Fatalf("b3 = %q, does not match %s", v, b3Re)
		}
	}
}

func TestGenerateTraceparentShape(t *testing.T) {
	for i := 0; i < 20; i++ {
		v := GenerateTraceparent()
		if !traceparentRe.MatchString(v) {
			t.Fatalf("traceparent = %q, does not match %s", v, traceparentRe)
		}
	}
}

func TestGenerateProjectIDShape(t *testing.T) {
	for i := 0; i < 20; i++ {
		v := GenerateProjectID()
		if !projectIDRe.MatchString(v) {
			t.Fatalf("project id = %q, does not match %s", v, projectIDRe)
		}
	}
}

// A template rule produces its value regardless of the prefix, length and
// charset it carries — those fields only describe a random string.
func TestTransformTemplateRule(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "b3", Mode: HeaderModeGenerate, Template: "b3", Length: 8, Prefix: "zz_"},
			{Name: "traceparent", Mode: HeaderModeGenerate, Template: "traceparent"},
			{Name: "x-opencode-project", Mode: HeaderModeGenerate, Template: "project_id"},
		},
	}
	h := http.Header{}
	Transform(h, "p", rule, NewStore())

	if !b3Re.MatchString(h.Get("B3")) {
		t.Fatalf("b3 = %q", h.Get("B3"))
	}
	if !traceparentRe.MatchString(h.Get("Traceparent")) {
		t.Fatalf("traceparent = %q", h.Get("Traceparent"))
	}
	if !projectIDRe.MatchString(h.Get("X-Opencode-Project")) {
		t.Fatalf("project = %q", h.Get("X-Opencode-Project"))
	}
}

// The two tracing formats of the emulated client are derived from one trace
// context: the trace id and the span id are the same in either of them, so a
// request can be correlated across the two.
func TestTransformSharesTraceContextBetweenFormats(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "b3", Mode: HeaderModeGenerate, Template: "b3"},
			{Name: "traceparent", Mode: HeaderModeGenerate, Template: "traceparent"},
		},
	}
	h := http.Header{}
	Transform(h, "p", rule, NewStore())

	b3 := h.Get("B3")
	tp := h.Get("Traceparent")
	b3Parts := strings.Split(b3, "-")
	tpParts := strings.Split(tp, "-")
	if len(b3Parts) != 4 || len(tpParts) != 4 {
		t.Fatalf("unexpected shapes: b3=%q traceparent=%q", b3, tp)
	}
	if b3Parts[0] != tpParts[1] {
		t.Fatalf("trace id differs: b3=%q traceparent=%q", b3, tp)
	}
	if b3Parts[1] != tpParts[2] {
		t.Fatalf("span id differs: b3=%q traceparent=%q", b3, tp)
	}
}

// Two requests must not share a trace context: the values are per request.
func TestTransformGivesEachRequestItsOwnTraceContext(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "b3", Mode: HeaderModeGenerate, Template: "b3"},
			{Name: "traceparent", Mode: HeaderModeGenerate, Template: "traceparent"},
		},
	}
	first := http.Header{}
	Transform(first, "p", rule, NewStore())
	second := http.Header{}
	Transform(second, "p", rule, NewStore())

	if first.Get("B3") == second.Get("B3") {
		t.Fatal("two requests got the same trace context")
	}
}

// A template that is not known is rejected by the rule validation, so a typo
// in an extension fails loudly instead of silently sending a random string.
func TestValidateRuleRejectsUnknownTemplate(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "b3", Mode: HeaderModeGenerate, Template: "nope"},
		},
	}
	if err := ValidateRule("p", rule); err == nil {
		t.Fatal("expected an unknown template to be rejected")
	}
}

// A template only makes sense for the generating modes.
func TestValidateRuleRejectsTemplateWithStaticMode(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "b3", Mode: HeaderModeStatic, Value: "x", Template: "b3"},
		},
	}
	if err := ValidateRule("p", rule); err == nil {
		t.Fatal("expected a template with mode static to be rejected")
	}
}
