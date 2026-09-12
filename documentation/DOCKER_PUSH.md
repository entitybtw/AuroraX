# Building & publishing the Docker image

The official published image for this fork is **`entbtw/aurora`** on Docker Hub.

**Current published tags:** `latest`, `v1.0.3`, `v1.0.1`, and `v1.0.0` (linux/amd64). Pull it with:

```bash
docker pull entbtw/aurora:latest
docker pull entbtw/aurora:v1.0.3
docker pull entbtw/aurora:v1.0.1
docker pull entbtw/aurora:v1.0.0
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
  -t entbtw/aurora:v1.0.3 \
  --build-arg VERSION=1.0.3 \
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

## Verify

```bash
docker run --rm entbtw/aurora:latest --help
# and
docker run -d --name aurora -e AURORA_MASTER_KEY=sk-... entbtw/aurora:latest
```

## Tag conventions

Current published tags: `latest` and a pinned `vX.Y.Z`. Add a tag by adding `-t entbtw/aurora:vX.Y.Z` to the build command, or re-tag an existing image:

```bash
docker tag entbtw/aurora:latest entbtw/aurora:v1.0.0
docker push entbtw/aurora:v1.0.0
```
