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
# Do not pin the apk revision here: Alpine rotates package revisions
# within a release branch, which breaks Docker builds over time.
RUN apk add --no-cache ca-certificates

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and cross-compile for the target platform
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

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary and runtime config
COPY --from=builder /aurora /aurora
COPY --from=builder /app/configs/*.yaml /app/configs/

# Create writable .cache and data directories for nonroot user (UID=65532)
COPY --from=builder --chown=65532:65532 /app/.cache /app/.cache
COPY --from=builder --chown=65532:65532 /app/data /app/data

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["/aurora"]
