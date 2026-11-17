# TLS front (fingerprint presets)

The sidecar that emulates the upstream's own client runs on **Bun**, and Bun
offers no way to change its TLS ClientHello. Every request therefore left with
the same fingerprint, and nothing in the sidecar could vary it.

The gateway now ships a small **TLS front**: a loopback HTTP server that accepts
the sidecar's plain-HTTP request, applies the egress exit the gateway picked for
that attempt, and then opens the upstream connection itself with
`refraction-networking/utls` using a configurable fingerprint preset. Presets
rotate **per request**.

```
client → gateway → sidecar (Bun, signed body) → 127.0.0.1:8091 (TLS front)
                                                    └─ uTLS preset N → upstream
```

## Enabling it

The front is off unless the gateway is told to listen:

| Variable | Meaning |
|---|---|
| `AURORA_TLS_FRONT_LISTEN` | listen address, e.g. `127.0.0.1:8091`. Unset = front disabled. |
| `AURORA_TLS_FRONT_PRESETS` | comma-separated rotation order, e.g. `chrome120,firefox120,safari,edge`. Empty = `chrome120` only. |

The sidecar opts in through its own override file, `configs/sidecar-overrides.json`:

```json
{
  "tls_front_url": "http://127.0.0.1:8091"
}
```

The file is re-read when its mtime changes, so enabling or disabling the front
needs no restart — but the gateway must be running with
`AURORA_TLS_FRONT_LISTEN` set.

In front mode the sidecar:

- sends the signed request to the front instead of the upstream;
- adds `X-Aurora-Upstream: <real base_url>` so the front knows where to go;
- forwards `X-Aurora-Bind-Ip` / `X-Aurora-Exit-Proxy` to the front instead of
  using them as a fetch proxy — the front owns the exit now;
- applies the same routing to `/v1/models`, so chat and model listing share one
  fingerprint story.

## Presets

Names come from `httpclient.UTLSPresets()`:

`360`, `android`, `chrome`, `chrome115pq`, `chrome120`, `chrome120pq`, `edge`,
`firefox`, `firefox120`, `ios`, `qq`, `randomized`, `randomized-alpn`, `safari`.

Rotation is round-robin in the configured order, so with
`chrome120,firefox120,safari,edge` the first request uses Chrome 120, the next
Firefox 120, and so on.

Every response carries `X-Aurora-TLS-Preset` with the fingerprint that served
it, which makes rotation verifiable with a single loop:

```bash
for i in 1 2 3 4; do
  curl -sS -D - -o /dev/null http://127.0.0.1:7841/v1/chat/completions \
    -H "Authorization: Bearer $AURORA_MASTER_KEY" -H 'Content-Type: application/json' \
    -d '{"model":"<model>","messages":[{"role":"user","content":"hi"}],"max_tokens":8}' \
  | grep -i x-aurora-tls-preset
done
```

## What it does not change

- **Signature/identity**: headers, body and tool schemas still come from the
  sidecar. The front only changes the TLS layer.
- **Egress**: the exit is still chosen by the gateway's egress registry; the
  front just dials it (source address or proxy) instead of the sidecar.
- **Quota**: the free-tier allowance is metered upstream; changing fingerprints
  does not create quota. On 2026-11-17 a Chrome 120 fingerprint was accepted
  and answered with the same `FreeUsageLimitError` as Bun, which is exactly the
  expected behaviour for a quota that is not per-address.

## Implementation notes

- `internal/http_client/utls_transport.go` — presets live here; the transport
  also gained proper request framing (`Content-Length` or chunked). Without it,
  HTTP/1.1 POSTs were written with a body and no length, and the upstream
  stalled then reset the connection.
- `internal/tlsfront/front.go` — the loopback server: `X-Aurora-Upstream`
  handling, exit application, per-request rotation, streaming with a flush per
  chunk so SSE answers are not buffered.
- `internal/application/app.go` — starts the front when
  `AURORA_TLS_FRONT_LISTEN` is set and closes it on shutdown.
- `internal/providers/sidecarclient/sidecar/{adapter,execute}.js` — front mode.

## Two operational details worth knowing

**The listen port is shared.** With `network_mode: host` the fixed loopback
port outlives the container: a previous container still shutting down holds it
for a few seconds, and a front left behind by a crash holds it forever.
Startup therefore does not fail on `EADDRINUSE`; it probes the port and
**reuses** the listener if it answers like a front, and only errors when the
port is held by something else. If the gateway refuses to start with
`tls front: listen ... address already in use (and the listener is not a
front)`, something unrelated owns the port — find it with
`ss -ltnp | grep <port>`.

**`base_url` may point at the front.** When it does, the sidecar takes the real
upstream from `AURORA_SIDECAR_UPSTREAM_REAL` instead of looping into the front,
and answers `503` with an explicit message when that variable is unset. The
simpler setup keeps `base_url` at the real upstream and only sets
`tls_front_url`; both work.
