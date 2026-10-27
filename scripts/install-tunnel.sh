#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
curl -fL --connect-timeout 15 --max-time 120 https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o "$RUNTIME/bin/cloudflared"
chmod 700 "$RUNTIME/bin/cloudflared"
"$RUNTIME/bin/cloudflared" --version
