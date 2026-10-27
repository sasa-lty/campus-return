#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/../scripts/env.sh"
pid="$(<"$RUNTIME/server.pid")"
target="$(readlink "/proc/$pid/exe")"
[[ "$target" == "$RUNTIME/bin/server" || "$target" == "$RUNTIME/bin/server (deleted)" ]] || { echo 'Unexpected server PID.' >&2; exit 1; }
kill -TERM "$pid"
for ((i=0;i<100;i++)); do [[ ! -e "/proc/$pid/exe" ]] && break; sleep .1; done
[[ ! -e "/proc/$pid/exe" ]] || { echo 'Web service did not stop safely.' >&2; exit 1; }
export BACKUP_PASSWORD="$(openssl rand -hex 20)"
printf 'export BACKUP_PASSWORD=%s\n' "$BACKUP_PASSWORD" > "$RUNTIME/backup-verification.env"
archive="$(bash "$ROOT/scripts/backup.sh" | sed -n 's/^Encrypted backup: //p')"
[[ -f "$archive" ]]
plain="$RUNTIME/backup-verification.tar.gz"
openssl enc -d -aes-256-cbc -pbkdf2 -pass env:BACKUP_PASSWORD -in "$archive" -out "$plain"
cd "$ROOT"
python3 tests/check_backup.py "$plain"
rm -- "$plain"
if bash scripts/restore.sh "$archive"; then echo 'Restore should refuse a populated environment.' >&2; exit 1; fi
export APP_MODE=fabric
exec bash scripts/run.sh
