#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
command -v cloudflared >/dev/null || { echo 'Install cloudflared first, then run this script again.' >&2; exit 1; }
curl --fail --silent http://127.0.0.1:8088/api/v1/health >/dev/null
exec cloudflared tunnel --url http://127.0.0.1:8088 --protocol http2 --edge-ip-version 4
