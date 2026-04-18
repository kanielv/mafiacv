#!/usr/bin/env bash
# Runs all mafiacv services locally for development:
#   - mcp-server binary (built, spawned by story-service as a child process)
#   - story-service (port 8090)
#   - backend        (port 8080)
#   - frontend (Vite dev server)
#
# Ctrl+C shuts everything down. Logs are prefixed per service.
#
# Requirements:
#   - go, node/npm on PATH
#   - GEMINI_API_KEY in story-service/.env (or exported)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

# Ensure go is visible (handles the common case where it's in /usr/local/go/bin
# but not in the PATH of the invoking shell).
if ! command -v go >/dev/null 2>&1; then
  export PATH="$PATH:/usr/local/go/bin"
fi

command -v go   >/dev/null 2>&1 || { echo "error: go not found on PATH"; exit 1; }
command -v npm  >/dev/null 2>&1 || { echo "error: npm not found on PATH"; exit 1; }

MCP_BIN="$ROOT/mcp-server/mcp-server"
MCP_DB="$ROOT/story-service/mcp-server.db"
export MCP_BINARY_PATH="$MCP_BIN"
export MCP_DB_PATH="$MCP_DB"
export STORY_SERVICE_URL="${STORY_SERVICE_URL:-http://localhost:8090}"

echo "==> building mcp-server binary"
(cd "$ROOT/mcp-server" && go build -o "$MCP_BIN" ./cmd/mcp-server)

PIDS=()
cleanup() {
  echo
  echo "==> shutting down"
  for pid in "${PIDS[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  # give children a beat, then force
  sleep 1
  for pid in "${PIDS[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

# Prefix every line of a background service's output with a colored tag.
run_tagged() {
  local tag="$1"; shift
  local color="$1"; shift
  ( "$@" 2>&1 | while IFS= read -r line; do
      printf '\033[%sm[%s]\033[0m %s\n' "$color" "$tag" "$line"
    done
  ) &
  PIDS+=("$!")
}

echo "==> starting story-service (port 8090)"
run_tagged "story " "36" bash -c "cd '$ROOT/story-service' && go run ./cmd/story"

echo "==> starting backend (port 8080)"
run_tagged "back  " "33" bash -c "cd '$ROOT/backend' && go run ./cmd/app"

echo "==> starting frontend (Vite)"
run_tagged "front " "35" bash -c "cd '$ROOT/frontend' && npm run dev"

echo "==> all services launched. Ctrl+C to stop."
wait
