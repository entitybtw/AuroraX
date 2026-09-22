# Combined build image: the compiled Aurora binary plus the Bun runtime.
#
# Split out from the main Dockerfile so the runtime image can consume it by tag.
# BuildKit can mis-cache cross-stage `COPY --from=builder` in a single build
# graph and ship a stale binary; a prebuilt image sidesteps that entirely.
#
# Usage:
#   docker build -f Dockerfile.builder -t entbtw/aurora-builder:$(git rev-parse --short HEAD) .
#   docker build -f Dockerfile.runtime --build-arg BUILDER_IMAGE=<tag> -t entbtw/aurora:<tag> .
FROM --platform=$BUILDPLATFORM node:22-alpine3.23 AS ui-builder

WORKDIR /app

COPY dashboard-ui ./dashboard-ui
RUN cd dashboard-ui && corepack enable && pnpm install --no-frozen-lockfile && pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

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

# --- Bun runtime stage -----------------------------------------------------
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
	chmod +x /bun/bun

# --- Combine into a single build artifact image ----------------------------
FROM alpine:3.23 AS combined

COPY --from=builder /aurora /aurora
COPY --from=builder /app/configs /app/configs
COPY --from=bun /bun/bun /bun/bun
