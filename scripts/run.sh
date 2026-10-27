#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
cd "$ROOT"
go build -o "$RUNTIME/bin/server" ./cmd/server
printf '%s\n' "$$" > "$RUNTIME/server.pid"
echo "Open http://127.0.0.1:8088 ; admin credentials are stored only in $RUNTIME/secrets.env"
exec "$RUNTIME/bin/server"
