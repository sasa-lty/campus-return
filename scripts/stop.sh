#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
docker compose -f "$ROOT/deployment/compose.yaml" stop
echo 'Only the new network was stopped. Ledger volumes and old project are retained. Stop the web terminal with Ctrl+C.'
