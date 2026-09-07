# Getting Started

Aurora is an OpenAI/Anthropic-compatible gateway that routes requests to many AI providers. This fork adds a quick-start path driven by config files plus a dashboard, and includes the Session Hub for advanced API-integration workflows.

## 1. Clone

```bash
git clone <your-fork-repo.git> aurora
cd aurora
```

## 2. Build

> Requires Docker (multi-stage `Dockerfile`) or the Go toolchain.

**With Docker:**
```bash
docker build -t aurora:local .
docker run -d --name aurora -p 8080:8080 -e AURORA_MASTER_KEY=sk-live-... aurora:local
```

**Local Go run:**
```bash
go build -o aurora ./apps/aurora
./aurora
```

## 3. Configure

Point Aurora at a config file, or rely on defaults + env vars.

- Set `AURORA_MASTER_KEY` — protects the admin API/dashboard.
- Provide at least one provider. Providers can be declared in `configs/config.yaml` or created from the dashboard.
- See `configs/config.example.yaml` for a full template.

The gateway reads several override files next to your config if present:

| File | Contents |
|------|----------|
| `dashboard-overrides.yaml` | General settings |
| `provider-overrides.json` | Providers created in the UI |
| `pool-overrides.json` | Load-balanced pools |
| `session-hub-rules.yaml` | Session Hub rules |

## 4. Use it

OpenAI-compatible endpoint:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-aurora-key",   # or the master key
)

r = client.chat.completions.create(
    model="opencode-zen/mimo-v2.5-free",
    messages=[{"role": "user", "content": "hello"}],
)
print(r.choices[0].message.content)
```

Dashboard: `http://localhost:8080/admin/dashboard`

## Next steps

- `DEPLOYMENT.md` — running it in production, state files, multi-IP host networking.
- `SESSION_HUB.md` — the header transformation & session mapping engine.
