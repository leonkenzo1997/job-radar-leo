#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

GO_BIN="${GO_BIN:-/Users/truongtangquang/sdk/go1.26.0/bin/go}"
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
fi

PORT="${PORT:-8090}"

echo "Building Leo Job Radar..."
"$GO_BIN" build -o job-radar .

echo "Starting server on http://localhost:${PORT} ..."
(sleep 1 && open "http://localhost:${PORT}") >/dev/null 2>&1 &
exec ./job-radar -port "$PORT"
