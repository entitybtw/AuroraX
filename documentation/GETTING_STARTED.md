# Getting Started with Aurora (this fork)

Aurora is an OpenAI/Anthropic-compatible AI gateway that routes requests to many providers. It is a single static binary. This fork ships a published Docker image, a config-driven + dashboard workflow, and the **Session Hub** for advanced API-integration setups.

The fastest path is pulling the published image — no source, no build.

## Option 1 — Easiest: pull the Docker image

```bash
docker pull entbtw/aurora:latest
docker run -d --name aurora -p 8080:8080 \
  -e AURORA_MASTER_KEY="your-secure-key" \
  entbtw/aurora:latest
```

Verify:
- API base: `http://localhost:8080/v1`
- Dashboard: `http://localhost:8080/admin/dashboard`
- Health: `curl http://localhost:8080/health`

### Add a provider

Providers come from your config, environment, or the dashboard. The quickest path is the dashboard:

1. Open **http://localhost:8080/admin/dashboard** → **Providers**.
2. **Add provider**: pick type (e.g. `vllm`), set `base_url`, paste an `api_key`, list models, optionally set `bind_ip`.
3. If you use several accounts behind one logical name, create a **Pool** from them (next step).

For config-driven setups see [DEPLOYMENT.md](DEPLOYMENT.md) (state files, host networking) and `configs/config.example.yaml`.

## Option 2 — Run from source

> Requires the Go toolchain (or Docker to use the multi-stage `Dockerfile`).

```bash
git clone <your-fork-repo.git> aurora
cd aurora

# With Go
go build -o aurora ./apps/aurora
AURORA_MASTER_KEY="your-secure-key" ./aurora

# Or with Docker
docker build -t aurora:local .
docker run -d --name aurora -p 8080:8080 -e AURORA_MASTER_KEY="your-secure-key" aurora:local
```

## Use the API

The gateway is OpenAI-compatible, so the standard `openai` SDK works:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-aurora-key",        # your managed key or the master key
)

r = client.chat.completions.create(
    model="opencode-zen/mimo-v2.5-free",   # "<pool-or-provider>/<model>"
    messages=[{"role": "user", "content": "hello"}],
)
print(r.choices[0].message.content)
```

Anthropic SDKs work the same way against `/v1/messages` — no format changes.

## Model selector format

`model` values are `<target>/<model>` where `target` is a pool or provider name
(e.g. `opencode-zen/mimo-v2.5-free`, `openrouter/gpt-oss-20b:free`). Naming a pool routes
round-robin across its members.

## Next steps

- [DEPLOYMENT.md](DEPLOYMENT.md) — production Docker, persistent state, multi-IP host networking.
- [MULTI_ACCOUNT.md](MULTI_ACCOUNT.md) — end-to-end load-balanced accounts with distinct, stable client identities.
- [SESSION_HUB.md](SESSION_HUB.md) — header transformation & per-account session mapping.
- [DOCKER_PUSH.md](DOCKER_PUSH.md) — the published image, tags, how to build/publish yourself.
