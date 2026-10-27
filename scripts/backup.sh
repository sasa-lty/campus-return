#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
[[ -n "${BACKUP_PASSWORD:-}" && ${#BACKUP_PASSWORD} -ge 12 ]] || { echo 'Set BACKUP_PASSWORD (at least 12 characters) before backup.' >&2; exit 1; }
if curl --silent --max-time 2 http://127.0.0.1:8088/api/v1/health >/dev/null; then echo 'Stop the web service before making a full ledger backup.' >&2; exit 1; fi
work="$(mktemp -d "$RUNTIME/backup.XXXXXX")"
cleanup(){ [[ "$work" == "$RUNTIME"/backup.* ]] && rm -rf -- "$work"; }
trap cleanup EXIT
mkdir -p "$work/data"
(cd "$ROOT" && go run ./cmd/maintenance backup "$work/data/platform.db")
if [[ -d "$DATA_DIR/uploads" ]]; then cp -a "$DATA_DIR/uploads" "$work/data/"; fi
docker compose -f "$ROOT/deployment/compose.yaml" stop
trap 'docker compose -f "$ROOT/deployment/compose.yaml" start >/dev/null; cleanup' EXIT
cp -a "$FABRIC_ROOT/organizations" "$work/organizations"
cp "$FABRIC_ROOT/channel.block" "$work/channel.block"
for volume in campus-return_orderer-data campus-return_peer1-data campus-return_peer2-data; do
  docker volume inspect "$volume" >/dev/null
  docker run --rm --network none -v "$volume:/source:ro" -v "$work:/backup" busybox tar -czf "/backup/$volume.tgz" -C /source .
done
output="$RUNTIME/backup-$(date -u +%Y%m%dT%H%M%SZ).tar.gz.enc"
tar -czf - -C "$work" . | openssl enc -aes-256-cbc -pbkdf2 -salt -pass env:BACKUP_PASSWORD -out "$output"
echo "Encrypted backup: $output"
echo 'Keep secrets.env separately; it is required for decrypting private materials after restoration.'
