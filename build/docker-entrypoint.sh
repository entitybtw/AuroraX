#!/bin/sh
# Container entrypoint: start the free tier sidecar (Bun) and, when
# configured, one bind-proxy per egress IP, then run the Aurora gateway.
#
# The sidecar is optional. Set AURORA_SIDECAR_ENABLED=false to skip it.
# Multi-IP: set OPENCODE_ZEN_BIND_IPS to a comma-separated list of local IPs
# (e.g. "203.0.113.10,203.0.113.11"). The entrypoint starts an Aurora
# bind-proxy for each on ports 8981+ and points the sidecar at them so zen
# requests egress from the matching provider bind_ip.
set -eu

SIDECAR_ENABLED="${AURORA_SIDECAR_ENABLED:-true}"
SIDECAR_PORT="${AURORA_SIDECAR_PORT:-8090}"
SIDECAR_DIR="${AURORA_SIDECAR_DIR:-/opt/sidecarproxy}"
BIND_IPS="${OPENCODE_ZEN_BIND_IPS:-}"

BIND_PROXIES=""
BASE_PROXY_PORT=8981

if [ -n "$BIND_IPS" ]; then
	port="$BASE_PROXY_PORT"
	old_ifs="$IFS"
	IFS=','
	for ip in $BIND_IPS; do
		ip="$(echo "$ip" | tr -d ' ')"
		[ -z "$ip" ] && continue
		BIND_IP="$ip" PROXY_LISTEN="127.0.0.1:${port}" /aurora bindproxy &
		if [ -n "$BIND_PROXIES" ]; then
			BIND_PROXIES="${BIND_PROXIES},${ip}:${port}"
		else
			BIND_PROXIES="${ip}:${port}"
		fi
		echo "bind-proxy started: ${ip} -> 127.0.0.1:${port}"
		port=$((port + 1))
	done
	IFS="$old_ifs"
	export OPENCODE_ZEN_BIND_PROXIES="$BIND_PROXIES"
fi

if [ "$SIDECAR_ENABLED" = "true" ] && [ -f "$SIDECAR_DIR/adapter.js" ]; then
	/usr/local/bin/bun "$SIDECAR_DIR/adapter.js" &
	SIDECAR_PID=$!

	i=0
	while [ "$i" -lt 20 ]; do
		if /usr/local/bin/bun -e "process.exit((await fetch('http://127.0.0.1:${SIDECAR_PORT}/health')).ok ? 0 : 1)" >/dev/null 2>&1; then
			break
		fi
		i=$((i + 1))
		sleep 0.5
	done

	trap 'kill "$SIDECAR_PID" 2>/dev/null || true' TERM INT
fi

exec /aurora "$@"
