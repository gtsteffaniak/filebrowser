#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VITE_PORT="${VITE_DEV_PORT:-5173}"
VITE_ORIGIN="${VITE_DEV_ORIGIN:-http://localhost:8080}"
VITE_CLIENT_PORT="${VITE_DEV_CLIENT_PORT:-8080}"
READY_URL="http://127.0.0.1:${VITE_PORT}/__vite/@vite/client"
WAIT_SECONDS="${VITE_READY_TIMEOUT:-60}"

stop_listeners_on_port() {
	local port=$1
	if command -v fuser >/dev/null 2>&1; then
		fuser -k "${port}/tcp" 2>/dev/null || true
	elif command -v lsof >/dev/null 2>&1; then
		local pids
		pids="$(lsof -ti :"${port}" 2>/dev/null || true)"
		if [[ -n "$pids" ]]; then
			kill -TERM $pids 2>/dev/null || true
		fi
	elif command -v ss >/dev/null 2>&1; then
		local pid
		pid="$(ss -tlnp 2>/dev/null | sed -n "s/.*:${port} .*pid=\\([0-9]*\\).*/\\1/p" | head -n1)"
		if [[ -n "$pid" ]]; then
			kill -TERM "$pid" 2>/dev/null || true
		fi
	fi
	sleep 0.3
}

cleanup() {
	local status=$?
	trap - EXIT INT TERM
	if [[ -n "${AIR_PID:-}" ]] && kill -0 "$AIR_PID" 2>/dev/null; then
		kill -TERM "$AIR_PID" 2>/dev/null || true
	fi
	if [[ -n "${VITE_PID:-}" ]] && kill -0 "$VITE_PID" 2>/dev/null; then
		kill -TERM "$VITE_PID" 2>/dev/null || true
	fi
	wait "$AIR_PID" 2>/dev/null || true
	wait "$VITE_PID" 2>/dev/null || true
	exit "$status"
}

wait_for_vite() {
	local elapsed=0
	while (( elapsed < WAIT_SECONDS )); do
		if curl -fsS -o /dev/null "$READY_URL" 2>/dev/null; then
			return 0
		fi
		if ! kill -0 "$VITE_PID" 2>/dev/null; then
			echo "Vite dev server exited before becoming ready." >&2
			return 1
		fi
		sleep 0.5
		elapsed=$((elapsed + 1))
	done
	echo "Timed out waiting for Vite at ${READY_URL}" >&2
	return 1
}

trap cleanup EXIT INT TERM

if curl -fsS -o /dev/null "$READY_URL" 2>/dev/null || ss -tln 2>/dev/null | grep -q ":${VITE_PORT} "; then
	echo "Clearing stale listener on port ${VITE_PORT}..."
	stop_listeners_on_port "$VITE_PORT"
fi

echo "Starting Vite dev server on 127.0.0.1:${VITE_PORT}..."
(
	cd "$ROOT/frontend"
	export VITE_DEV_ORIGIN="$VITE_ORIGIN"
	export VITE_DEV_PORT="$VITE_PORT"
	export VITE_DEV_CLIENT_PORT="$VITE_CLIENT_PORT"
	exec npm run dev
) &
VITE_PID=$!

wait_for_vite
echo "Vite is ready. Starting Air..."

(
	cd "$ROOT/backend"
	exec go tool air
) &
AIR_PID=$!

wait -n "$VITE_PID" "$AIR_PID"
echo "A dev process exited; stopping the other..." >&2
exit 1
