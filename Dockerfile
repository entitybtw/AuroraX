# syntax=docker/dockerfile:1

# UI build stage — build dashboard UI with Node.js
FROM --platform=$BUILDPLATFORM node:22-alpine3.23 AS ui-builder

WORKDIR /app

COPY dashboard-ui ./dashboard-ui
RUN cd dashboard-ui && corepack enable && pnpm install --no-frozen-lockfile && pnpm build

# Go build stage — run on the build host's native arch for speed, cross-compile for target
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /app

# Install ca-certificates for HTTPS requests.
RUN apk add --no-cache ca-certificates

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and cross-compile for the target platform
ARG SOURCE_CACHE_BUST=0
RUN echo "source cache bust: ${SOURCE_CACHE_BUST}"
COPY . .
COPY --from=ui-builder /app/internal/admin/dashboard/dist ./internal/admin/dashboard/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG GO_BUILD_TAGS=
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build \
	-tags="${GO_BUILD_TAGS}" \
	-ldflags="-s -w -X aurora/internal/version.Version=${VERSION} -X aurora/internal/version.Commit=${COMMIT} -X aurora/internal/version.Date=${DATE}" \
	-o /aurora ./apps/aurora

# Create .cache and data directories for runtime (with placeholder for COPY)
RUN mkdir -p /app/.cache /app/data && touch /app/.cache/.keep /app/data/.keep

# ---------------------------------------------------------------------------
# Runtime stage (default) — minimal distroless image. No Bun, no sidecar.
# Use this unless the OpenCode Zen Bun sidecar is required.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Copy binary and runtime config
COPY --from=builder /aurora /aurora
COPY --from=builder /app/configs/*.yaml /app/configs/

# Create writable .cache and data directories for nonroot user (UID=65532)
COPY --from=builder --chown=65532:65532 /app/.cache /app/.cache
COPY --from=builder --chown=65532:65532 /app/data /app/data

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/aurora"]

# ---------------------------------------------------------------------------
# Bun stage — downloads the Bun runtime used by the OpenCode Zen sidecar.
# The zen free tier fingerprints the TLS handshake and only accepts Bun's
# BoringSSL ClientHello, which a Go binary cannot reproduce.
# ---------------------------------------------------------------------------
FROM alpine:3.23 AS bun

ARG TARGETARCH
ARG BUN_VERSION=1.3.14
RUN apk add --no-cache curl unzip ca-certificates && \
	case "${TARGETARCH}" in \
		amd64) ARCH=x64 ;; \
		arm64) ARCH=aarch64 ;; \
		*) echo "unsupported arch: ${TARGETARCH}" && exit 1 ;; \
	esac && \
	curl -fsSL -o /tmp/bun.zip "https://github.com/oven-sh/bun/releases/download/bun-v${BUN_VERSION}/bun-linux-${ARCH}.zip" && \
	mkdir -p /bun && \
	unzip -oj /tmp/bun.zip "bun-linux-${ARCH}/bun" -d /bun && \
	chmod +x /bun/bun && \
	ls -la /bun/bun

# ---------------------------------------------------------------------------
# Runtime stage with OpenCode Zen sidecar — debian-slim + Bun.
# Build with: docker build --target runtime-sidecar ...
# ---------------------------------------------------------------------------
FROM debian:bookworm-slim AS runtime-sidecar

# ca-certificates lets the sidecar verify upstream TLS certificates.
RUN apt-get update && \
	apt-get install -y --no-install-recommends ca-certificates && \
	rm -rf /var/lib/apt/lists/*

# Copy the Bun runtime and the sidecar for the OpenCode free tier.
COPY --from=bun /bun/bun /usr/local/bin/bun
COPY internal/providers/sidecarclient/sidecar /opt/sidecar

# Copy binary and runtime config. The cache-bust arg keeps BuildKit from
# serving a stale cross-stage COPY when the builder's /aurora changes.
ARG SOURCE_CACHE_BUST=0
RUN echo "runtime source cache bust: ${SOURCE_CACHE_BUST}"
COPY --from=builder /aurora /aurora
COPY --from=builder /app/configs/*.yaml /app/configs/

# Create writable .cache and data directories for the runtime user (UID=65532)
RUN mkdir -p /app/.cache /app/data && \
	chown -R 65532:65532 /app /opt/sidecar

COPY build/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Default to routing OpenCode through the local sidecar. Operators can set
# AURORA_SIDECAR_ENABLED=false to disable it at runtime.
ENV AURORA_SIDECAR_ENABLED=true \
	AURORA_SIDECAR_PORT=8090 \
	AURORA_SIDECAR_BASE_URL=http://127.0.0.1:8090/v1

USER 65532:65532

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/docker-entrypoint.sh"]
