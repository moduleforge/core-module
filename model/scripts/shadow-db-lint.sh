#!/usr/bin/env bash
# shadow-db-lint.sh — apply and revert migrations against an ephemeral
# Postgres container.
#
# Replaces `atlas migrate lint --dev-url …`. Spins up a throwaway Postgres
# instance, runs every migration in $1 against it via `goose up`, then rolls
# every migration back via `goose down-to 0` (exercising each migration's
# down script), and tears down regardless of outcome. Exit code matches the
# exit code of whichever goose invocation failed first (up, then down); 0
# only if both succeed.
#
# Usage:  shadow-db-lint.sh <migrations-dir>
#
# Connectivity: the container is started with its 5432 also published to a
# random loopback port, so two connection paths are available:
#   1. The container's Docker bridge IP (preferred — avoids host port
#      forwarding, works in sandboxed CI environments that route bridge IPs).
#   2. A published-port connection via 127.0.0.1 (fallback — needed under
#      Docker Desktop for macOS, where the bridge network runs inside a Linux
#      VM and its IPs are not routable from the host).
# The script probes the bridge IP first and falls back to the published port
# only if that probe fails, so environments where bridge-IP networking works
# see no behavior change.

set -u
set -o pipefail

DIR="${1:-migrations}"

if [[ ! -d "$DIR" ]]; then
  echo "shadow-db-lint: directory not found: $DIR" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "shadow-db-lint: docker is required" >&2
  exit 2
fi

if ! command -v goose >/dev/null 2>&1; then
  echo "shadow-db-lint: goose is required (go install github.com/pressly/goose/v3/cmd/goose@latest)" >&2
  exit 2
fi

CNAME="goose-lint-$$-$(date +%s)"
trap 'docker rm -f "$CNAME" >/dev/null 2>&1 || true' EXIT

echo "shadow-db-lint: starting ephemeral Postgres ($CNAME)..."
if ! docker run -d --rm --name "$CNAME" \
       -e POSTGRES_PASSWORD=lint \
       -p 127.0.0.1::5432 \
       postgres:16 >/dev/null; then
  echo "shadow-db-lint: failed to start container" >&2
  exit 2
fi

# Resolve container IP (preferred connection path — see header comment).
IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CNAME")
if [[ -z "$IP" ]]; then
  echo "shadow-db-lint: could not resolve container IP" >&2
  exit 2
fi

# Resolve the published host port (fallback connection path).
PUBLISHED=$(docker port "$CNAME" 5432/tcp 2>/dev/null | head -n1)
HOST_PORT="${PUBLISHED##*:}"

echo "shadow-db-lint: waiting for Postgres at ${IP}:5432..."
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if docker exec "$CNAME" pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker exec "$CNAME" pg_isready -U postgres >/dev/null 2>&1; then
  echo "shadow-db-lint: Postgres did not become ready in time" >&2
  exit 2
fi

# Probe host-side TCP reachability of a given host:port within a short
# timeout. Uses bash's /dev/tcp pseudo-device rather than an external tool
# so the script keeps its existing dependency set (docker + goose only).
tcp_reachable() {
  local host="$1" port="$2"
  timeout 3 bash -c ": >/dev/tcp/${host}/${port}" >/dev/null 2>&1
}

if tcp_reachable "$IP" 5432; then
  echo "shadow-db-lint: bridge IP ${IP}:5432 is reachable; using it"
  URL="postgres://postgres:lint@${IP}:5432/postgres?sslmode=disable"
elif [[ -n "$HOST_PORT" ]] && tcp_reachable "127.0.0.1" "$HOST_PORT"; then
  echo "shadow-db-lint: bridge IP ${IP}:5432 unreachable from host; falling back to published port 127.0.0.1:${HOST_PORT}"
  URL="postgres://postgres:lint@127.0.0.1:${HOST_PORT}/postgres?sslmode=disable"
else
  echo "shadow-db-lint: neither bridge IP (${IP}:5432) nor published port (127.0.0.1:${HOST_PORT:-unknown}) is reachable from host" >&2
  exit 2
fi

echo "shadow-db-lint: applying $DIR via goose (up)..."
goose -dir "$DIR" postgres "$URL" up
RC=$?

if [[ $RC -ne 0 ]]; then
  echo "shadow-db-lint: goose up failed (exit $RC)" >&2
  exit $RC
fi

echo "shadow-db-lint: reverting $DIR via goose (down)..."
goose -dir "$DIR" postgres "$URL" down-to 0
RC=$?

if [[ $RC -ne 0 ]]; then
  echo "shadow-db-lint: goose down failed (exit $RC)" >&2
  exit $RC
fi

echo "shadow-db-lint: ok"
exit 0
