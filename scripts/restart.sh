#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
for proc in /proc/[0-9]*; do
  target="$(readlink "$proc/exe" 2>/dev/null || true)"
  if [[ "$target" == "$RUNTIME/bin/server" || "$target" == "$RUNTIME/bin/server (deleted)" ]]; then
    kill -TERM "${proc##*/}"
    for ((i=0;i<30;i++)); do [[ ! -e "$proc/exe" ]] && break; sleep .1; done
    if [[ -e "$proc/exe" ]]; then kill -KILL "${proc##*/}"; fi
  fi
done
export APP_MODE="${APP_MODE:-fabric}"
exec bash "$ROOT/scripts/run.sh"
