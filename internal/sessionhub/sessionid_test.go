package sessionhub

import (
	"encoding/hex"
	"net/http"
	"regexp"
	"testing"
	"time"
)

// recoverMS inverts TimestampedHead: given a head it finds the millisecond that
// produced it, choosing the occurrence closest to `near`.
func recoverMS(t *testing.T, head string, near int64) int64 {
	t.Helper()

	raw, err := hex.DecodeString(head)
	if err != nil || len(raw) != 6 {
		t.Fatalf("head %q is not 6 hex bytes: %v", head, err)
	}
	var u uint64
	for _, b := range raw {
		u = u<<8 | uint64(b)
	}

	const mod = int64(1) << 48
	// n = ms<<12 + seq, a = -n-1, u = a mod 2^48
	// => n ≡ -u-1 (mod 2^48)
	nMod := ((-int64(u) - 1) % mod + mod) % mod

	// The counter is always small (first id of a millisecond in practice), so
	// try the plausible values and pick the ms nearest `near`.
	var best int64
	bestDelta := int64(1) << 62
	for seq := int64(1); seq <= 16; seq++ {
		r := ((nMod-seq)%mod + mod) % mod
		if r%(1<<12) != 0 {
			continue
		}
		msMod := r / (1 << 12) // ms mod 2^36
		const step = int64(1) << 36
		k := (near - msMod) / step
		for _, kk := range []int64{k - 1, k, k + 1} {
			ms := msMod + kk*step
			if ms < 0 {
				continue
			}
			if d := abs64(ms - near); d < bestDelta {
				best, bestDelta = ms, d
			}
		}
	}
	if bestDelta > 60_000 {
		t.Fatalf("head %q decoded to a time %dms away from %d — not a timestamp", head, bestDelta, near)
	}
	return best
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestTimestampedHeadRoundTrip(t *testing.T) {
	for _, offset := range []time.Duration{0, time.Second, time.Minute, time.Hour, 24 * time.Hour} {
		now := time.Now().Add(offset)
		head := TimestampedHead(now)
		if len(head) != timestampedHeadLen {
			t.Fatalf("head %q length = %d, want %d", head, len(head), timestampedHeadLen)
		}
		got := recoverMS(t, head, now.UnixMilli())
		if abs64(got-now.UnixMilli()) > 2 {
			t.Errorf("head %q decoded to %d, want ~%d", head, got, now.UnixMilli())
		}
	}
}

// The head is the complement of the clock, so it must *decrease* as time
// advances. A random hex prefix has no such property and is what gets rejected.
func TestTimestampedHeadDecreasesWithTime(t *testing.T) {
	base := time.Now()
	a := TimestampedHead(base)
	b := TimestampedHead(base.Add(time.Second))
	c := TimestampedHead(base.Add(2 * time.Second))

	if b >= a {
		t.Errorf("head did not decrease over 1s: %s -> %s", a, b)
	}
	if c >= b {
		t.Errorf("head did not decrease over 2s: %s -> %s", b, c)
	}
}

func TestGenerateTimestampedIDShape(t *testing.T) {
	id := GenerateTimestampedID(26)
	if len(id) != 26 {
		t.Fatalf("length = %d, want 26 (%q)", len(id), id)
	}
	re := regexp.MustCompile(`^[0-9a-f]{12}[0-9A-Za-z]{14}$`)
	if !re.MatchString(id) {
		t.Fatalf("id %q does not match 12 hex + 14 base62", id)
	}
	// the whole point: the head must be a live timestamp
	got := recoverMS(t, id[:timestampedHeadLen], time.Now().UnixMilli())
	if abs64(got-time.Now().UnixMilli()) > 50 {
		t.Errorf("id %q head decodes to %d, want now", id, got)
	}
}

func TestGenerateValueTimestampedCharset(t *testing.T) {
	v := GenerateValue("ses_", 26, "timestamped")
	if len(v) != 30 || v[:4] != "ses_" {
		t.Fatalf("value = %q, want ses_ + 26 chars", v)
	}
	re := regexp.MustCompile(`^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
	if !re.MatchString(v) {
		t.Fatalf("value %q does not match timestamped session shape", v)
	}
	// the whole point: it must decode to "now", not to a random instant
	recoverMS(t, v[4:16], time.Now().UnixMilli())
}

func TestNormalizeCharsetKeepsTimestamped(t *testing.T) {
	if got := NormalizeCharset("Timestamped"); got != CharsetTimestamped {
		t.Errorf("NormalizeCharset(Timestamped) = %q, want %q", got, CharsetTimestamped)
	}
	if got := NormalizeCharset("hex"); got != CharsetHex {
		t.Errorf("NormalizeCharset(hex) = %q", got)
	}
	if got := NormalizeCharset(""); got != CharsetAlphanumeric {
		t.Errorf("NormalizeCharset() = %q", got)
	}
}

func TestValidateRuleAcceptsTimestampedCharset(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-timestamped-session", Mode: HeaderModeMapOrGenerate, Prefix: "ses_", Length: 26, Charset: CharsetTimestamped},
		},
	}
	if err := ValidateRule("timestamped", rule); err != nil {
		t.Fatalf("ValidateRule rejected timestamped charset: %v", err)
	}
}

func TestTransformGeneratesDecodableSession(t *testing.T) {
	rule := ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{
			{Name: "x-timestamped-session", Mode: HeaderModeGenerate, Prefix: "ses_", Length: 26, Charset: CharsetTimestamped},
			{Name: "x-session-id", Mode: HeaderModeGenerate, Prefix: "ses_", Length: 26, Charset: CharsetTimestamped},
		},
	}
	out := Transform(http.Header{}, "timestamped", rule, nil)
	if len(out) != 2 {
		t.Fatalf("generated %d headers, want 2", len(out))
	}
	for name, v := range out {
		if len(v) != 30 {
			t.Fatalf("%s = %q, want 30 chars", name, v)
		}
		recoverMS(t, v[4:16], time.Now().UnixMilli())
	}
}
