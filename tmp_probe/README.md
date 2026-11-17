# Fingerprint / quota probe (throwaway)

Black-box probes used on 2026-11-17 to find what the upstream's free-tier
meter is anchored to. Not part of the gateway; safe to delete.

- `probe.py` — suites `ips`, `models`, `tokens`, `fingerprint`, `headless`,
  `session`; runs on the VPS against `/root/probe`.
- `main.go` — the uTLS request probe binary (`go build -o probe ./tmp_probe`).

Result of the run (see the session report): address, token, identity headers,
session shape, tool schema and TLS preset all change nothing; every chat call
returns the same `FreeUsageLimitError`, while `/models` keeps answering 200 from
every address. The window is therefore metered above the address and below the
request shape, and lifting it needs working credentials rather than a different
client.
