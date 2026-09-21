#!/bin/sh
# Container entrypoint: start the OpenCode Zen sidecar (Bun) in the background,
# then run the Aurora gateway in the foreground.
#
# The sidecar is optional. Set AURORA_SIDECAR_ENABLED=false to skip it,
# or point OPENCODE_ZEN_BASE_URL at a different upstream.
set -eu

SIDECAR_ENABLED="${AURORA_SIDECAR_ENABLED:-true}"
SIDECAR_PORT="${AURORA_SIDECAR_PORT:-8090}"
SIDECAR_DIR="${AURORA_SIDECAR_DIR:-/opt/zenproxy}"

if [ "$SIDECAR_ENABLED" = "true" ] && [ -f "$SIDECAR_DIR/adapter.js" ]; then
	/usr/local/bin/bun "$SIDECAR_DIR/adapter.js" &
	SIDECAR_PID=$!

	# Wait for the sidecar to answer its health probe (up to ~10s).
	i=0
	while [ "$i" -lt 20 ]; do
		if /usr/local/bin/bun -e "process.exit((await fetch('http://127.0.0.1:${SIDECAR_PORT}/health')).ok ? 0 : 1)" >/dev/null 2>&1; then
			break
		fi
		i=$((i + 1))
		sleep 0.5
	done

	# Stop the whole container if the sidecar dies.
	trap 'kill "$SIDECAR_PID" 2>/dev/null || true' TERM INT
fi

exec /aurora "$@"
