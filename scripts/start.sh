#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
bash "$ROOT/scripts/network.sh"
bash "$ROOT/scripts/deploy.sh"
export APP_MODE=fabric
exec bash "$ROOT/scripts/run.sh"
