# Building & publishing the Docker image

The official published image for this fork is **`entbtw/aurora`** on Docker Hub.

**Current published tags:** `latest`, `v1.1.2`, `v1.0.4`, `v1.0.3`, `v1.0.2`, `v1.0.1`, and `v1.0.0` (linux/amd64). Pull it with:

```bash
docker pull entbtw/aurora:latest
docker pull entbtw/aurora:v1.1.2
```

The `Dockerfile` is multi-stage: it builds the React dashboard, cross-compiles the Go binary, and copies it into a distroless runtime image. This document covers building and pushing your own builds.

> Building multi-platform requires Docker **Buildx** (BuildKit). Enable it either via `docker buildx` (Docker 23+) or install the plugin (see below if `docker buildx` is unknown).

## Ensure Buildx is available

```bash
docker buildx version
```

If it reports `docker: unknown command: docker buildx`, install the plugin:

```bash
# amd64 host
curl -sSL -o ~/.docker/cli-plugins/docker-buildx \
  https://github.com/docker/buildx/releases/download/v0.37.0/buildx-v0.37.0.linux-amd64
chmod +x ~/.docker/cli-plugins/docker-buildx
docker buildx version
```

## Login to Docker Hub

```bash
docker login
```

Use your Docker Hub username and an **access token** (Account Settings → Security → New Access Token, scope Read & Write).

## Build & push (single arch)

```bash
docker buildx build --platform linux/amd64 \
  -t entbtw/aurora:latest \
  -t entbtw/aurora:v1.1.2 \
  --build-arg VERSION=1.1.2 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --progress=plain \
  --push .
```

Drop `--push` and add `--load` to build only into the local daemon (no push). Use `--platform linux/amd64,linux/arm64` for multi-arch; add `linux/arm/v7` if needed.

## Build args

| Arg | Default | Purpose |
|-----|---------|---------|
| `VERSION` | `dev` | Version baked into `version.Version`. |
| `COMMIT` | `none` | Git commit id. |
| `DATE` | `unknown` | Build date. |
| `GO_BUILD_TAGS` | (empty) | Extra Go build tags. |

## Image targets

| Target | Contents |
|--------|----------|
| `runtime` (default) | Distroless runtime: static binary + dashboard + sidecar sources (sidecar disabled by default). |
| `runtime-sidecar` | debian-slim + **Bun** sidecar + `aurora bindproxy`; enable with `AURORA_SIDECAR_ENABLED=true` (legacy: `AURORA_SIDECAR_*`). Target name kept for compatibility; content is a generic extension-driven sidecar (~100 MB larger). |

```bash
# Sidecar-enabled variant
docker buildx build --target runtime-sidecar \
  -t entbtw/aurora:sidecar --push .
```

### BuildKit stale-cache caveat (cross-stage COPY)

When using cached BuildKit builders, a multi-stage `COPY --from=builder` can pick up a **stale** binary from cache even though the source changed. To force a fresh binary, bust the cache (e.g. `--build-arg SOURCE_CACHE_BUST=$(date +%s)`), or use the local-artifact path:

```bash
# 1. Build artifacts locally
(cd dashboard-ui && bun run build)
CGO_ENABLED=0 go build -ldflags="-s -w" -o build/aurora ./apps/aurora

# 2. Build from local artifacts (Dockerfile.runtime copies build/ + dashboard dist)
docker buildx build -f Dockerfile.runtime --target runtime-sidecar \
  -t entbtw/aurora:sidecar --push .
```

Verify no stale binary shipped:

```bash
docker run --rm --entrypoint grep entbtw/aurora:sidecar -a \
  /app/aurora -l "bindproxy"   # should match
```

## Verify

```bash
docker run --rm entbtw/aurora:latest --help
# and
docker run -d --name aurora -e AURORA_MASTER_KEY=sk-... entbtw/aurora:latest
```

### Docker Compose: `working_dir` is required

The distroless runtime image defaults the working directory to `/home/nonroot`. The binary reads config files (`configs/provider-overrides.json`, `configs/pool-overrides.json`, etc.) relative to `/app`. **Always set `working_dir: /app` in your docker-compose.yml**, otherwise provider overrides silently fail to load on startup:

```yaml
services:
  aurora:
    image: entbtw/aurora:latest
    working_dir: /app      # REQUIRED
    volumes:
      - ./aurora-data:/app/data
      - ./configs:/app/configs
    # ...
```

## Tag conventions

Current published tags: `latest` and a pinned `vX.Y.Z`. Add a tag by adding `-t entbtw/aurora:vX.Y.Z` to the build command, or re-tag an existing image:

```bash
docker tag entbtw/aurora:latest entbtw/aurora:v1.1.2
docker push entbtw/aurora:v1.1.2
```
