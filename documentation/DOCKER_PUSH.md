# Building & publishing the Docker image

The official published image for this fork is **`entbtw/aurora`** on Docker Hub.

**Current release:** `v1.7.4` (also published as `latest`). Pull it with:

```bash
docker pull entbtw/aurora:latest
docker pull entbtw/aurora:v1.7.4
```

Every published tag is pinned: `vX.Y.Z` maps one-to-one to a git tag, and the
version string reported by the binary matches the tag:

```bash
docker exec aurora-gateway /aurora --version
# aurora [v1.7.4 | commit:<short> | go1.26.4]
```

### Versioning

- Patch releases within a minor series: `v1.7.0` → `v1.7.1` → … → `v1.7.9`, then `v1.8.0`.
- The patch series **stops before `.10`**: the release after `v1.6.9` is
  `v1.7.0`, never `v1.6.10`.
- `latest` always tracks the newest release tag.

The `Dockerfile` is multi-stage: it builds the React dashboard, cross-compiles the
Go binary, and copies both into a runtime image together with the sidecar and the
`bindproxy` helper. This document covers building, publishing and deploying your
own builds.

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

The version string carries the `v` prefix so it matches the git tag and the
image tag:

```bash
VERSION=v1.7.4   # next release: bump to v1.7.5

docker buildx build --platform linux/amd64 \
  -t entbtw/aurora:latest \
  -t entbtw/aurora:$VERSION \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --progress=plain \
  --push .
```

Drop `--push` and add `--load` to build only into the local daemon (no push). Use `--platform linux/amd64,linux/arm64` for multi-arch; add `linux/arm/v7` if needed.

## Release flow

A release is four steps; the tag is what the image is named after.

Stage changed files explicitly — `.state` and `hwparam.json` are local state
and must never be committed.

```bash
# 1. commit
git add changed-file changed-file && git commit

# 2. tag and push (both remotes)
git tag -a $VERSION -m "$VERSION"
git push origin main && git push github main
git push origin $VERSION && git push github $VERSION

# 3. build & push the image (command above)

# 4. deploy: pin the new tag, pull, recreate
cd /root/aurora-gateway
cp docker-compose.yml backups/docker-compose-$(date -u +%Y%m%d%H%M%S).yml
sed -i "s|image: entbtw/aurora:.*|image: entbtw/aurora:$VERSION|" docker-compose.yml
docker compose pull && docker compose up -d --force-recreate
```

`--force-recreate` matters: `docker compose up -d` alone leaves an existing
container running when only the tag changed, and the entrypoint (which starts
the sidecar and one `bindproxy` per source address) does not run again.

## Build args

| Arg | Default | Purpose |
|-----|---------|---------|
| `VERSION` | `dev` | Version baked into `version.Version`. |
| `COMMIT` | `none` | Git commit id. |
| `DATE` | `unknown` | Build date. |
| `GO_BUILD_TAGS` | (empty) | Extra Go build tags. |

## Image targets

The **last stage in the `Dockerfile` is the default**, so a plain
`docker buildx build … .` produces the sidecar image. Ask for `runtime`
explicitly when you want the smaller one.

| Target | Contents |
|--------|----------|
| `runtime-sidecar` (default) | debian-slim + **Bun** sidecar + `aurora bindproxy`, `ENTRYPOINT /docker-entrypoint.sh`. Starts one CONNECT proxy per source address and the sidecar, then execs the gateway. |
| `runtime` | Distroless: static binary + dashboard + sidecar sources, no Bun and no `bindproxy`, `ENTRYPOINT /aurora`. Extension-driven sidecar features are unavailable. |

```bash
# Distroless variant (no sidecar, no multi-IP rotation)
docker buildx build --target runtime \
  -t entbtw/aurora:distroless --push .
```

## Multi-IP egress (sidecar image only)

The entrypoint starts one `bindproxy` per address in `AURORA_SIDECAR_BIND_IPS`
on `127.0.0.1:8981+` and points the sidecar at them, so requests leave from a
different source address instead of one:

```bash
# .env next to docker-compose.yml
AURORA_SIDECAR_ENABLED=true
AURORA_SIDECAR_BIND_IPS=193.0.2.10,193.0.2.11,198.51.100.7
AURORA_SIDECAR_PORT=8090
```

Rotation is per request: consecutive requests walk the address list, and a
retry after a `403`/`429` moves to the next one. The gateway sends the
provider's primary address as `x-aurora-bind-ip`, but that is a preference,
not a pin — otherwise the extra addresses would be configured and unused.

The set is live: the gateway publishes the union of every provider's
`bind_ips` to `configs/sidecar-bind-proxies.json` (one CONNECT proxy per
address, the entrypoint's ones adopted rather than bound twice) and the
sidecar re-reads it, so adding an address to a provider makes it usable
immediately and dropping it from every provider takes it out of the
rotation.

Verify on a live host:

```bash
ss -lntp | grep -E ':8090|:898[1-6]'      # sidecar + one proxy per address
curl -s http://127.0.0.1:8090/health       # {"status":"ok"}
curl -s -x http://127.0.0.1:8982 https://api.ipify.org   # egress of address 2
```

Note that the container must use `network_mode: host` for the bind proxies to
bind real addresses; with a bridge network the source addresses are not on the
container's loopback and egress silently falls back to the default route.

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
docker tag entbtw/aurora:latest entbtw/aurora:v1.7.4
docker push entbtw/aurora:v1.7.4
```
