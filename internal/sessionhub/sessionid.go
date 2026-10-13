package sessionhub

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// timestampedAlphabet is timestamped's base62 alphabet, kept in its own order.
// Characters are picked with `byte % 62`, so the ordering must match exactly
// to reproduce the same distribution.
const timestampedAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// timestampedHeadLen is how many leading hex characters carry the timestamp
// (the high 48 bits of the complemented millisecond counter).
const timestampedHeadLen = 12

// timestampedDefaultLen is timestamped's own id length: 12 hex + 14 base62.
const timestampedDefaultLen = 26

// timestampedCounter mirrors the module-level `lastMs` / `seq` pair timestamped
// keeps: the sequence restarts at 1 on every new millisecond.
var timestampedCounter struct {
	sync.Mutex
	ms  int64
	seq int64
}

// TimestampedHead returns the 12-character time-derived prefix timestamped embeds
// in every session id, for the given instant.
//
// It reproduces timestamped's factory verbatim:
//
//	n := ms<<12 | seq      // seq restarts at 1 for each new millisecond
//	a := ~n                // BigInt complement (infinite two's complement)
//	head := big-endian hex of a's low 48 bits
//
// Because `a` is the *complement* of the clock, the head decreases as wall
// time advances — which is why a real timestamped session id always starts with
// "ec" and, for a first-in-millisecond id, ends in "ffe".
//
// Upstreams that gate free-tier access decode this head back to a timestamp
// and reject ids whose head does not resolve to a plausible "now", so a plain
// random hex prefix fails even though it matches the shape.
func TimestampedHead(now time.Time) string {
	ms := now.UnixMilli()

	timestampedCounter.Lock()
	if ms != timestampedCounter.ms {
		timestampedCounter.ms = ms
		timestampedCounter.seq = 0
	}
	timestampedCounter.seq++
	seq := timestampedCounter.seq
	timestampedCounter.Unlock()

	n := (ms << 12) + seq
	a := -n - 1 // BigInt ~n
	u := uint64(((a % (1 << 48)) + (1 << 48)) % (1 << 48))

	var raw [6]byte
	for d := 0; d < len(raw); d++ {
		raw[d] = byte(u >> (40 - 8*d))
	}
	return hex.EncodeToString(raw[:])
}

// GenerateTimestampedID returns timestamped's id body: a 12-hex time-derived head
// followed by random base62 characters, totalling `length` characters.
func GenerateTimestampedID(length int) string {
	if length <= timestampedHeadLen {
		length = timestampedDefaultLen
	}

	var b strings.Builder
	b.Grow(length)
	b.WriteString(TimestampedHead(time.Now()))

	tail := length - timestampedHeadLen
	buf := make([]byte, tail)
	// #nosec G115 -- index is taken modulo the alphabet length, same as the
	// upstream implementation.
	_, _ = rand.Read(buf)
	for _, c := range buf {
		b.WriteByte(timestampedAlphabet[int(c)%len(timestampedAlphabet)])
	}
	return b.String()
}
