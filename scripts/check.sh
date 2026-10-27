#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
cd "$ROOT"
go test -timeout 120s . ./backend/... ./cmd/... ./internal/... ./tests/...
go vet . ./backend/... ./cmd/... ./internal/... ./tests/...
for file in scripts/*.sh; do bash -n "$file"; done
curl --fail --silent http://127.0.0.1:8088/api/v1/health
echo
python3 tests/e2e.py
