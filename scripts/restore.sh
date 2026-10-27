#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RUNTIME="${CAMPUS_RETURN_RUNTIME:-$HOME/.local/share/campus-return}"
[[ $# == 1 && -f "$1" && -n "${BACKUP_PASSWORD:-}" ]] || { echo 'Usage: BACKUP_PASSWORD=... bash scripts/restore.sh BACKUP_FILE' >&2; exit 1; }
[[ -f "$RUNTIME/secrets.env" ]] || { echo 'First restore secrets.env separately to the new runtime directory.' >&2; exit 1; }
[[ ! -e "$RUNTIME/data/platform.db" ]] || { echo 'Refusing to overwrite an existing database.' >&2; exit 1; }
for volume in campus-return_orderer-data campus-return_peer1-data campus-return_peer2-data; do
  if docker volume inspect "$volume" >/dev/null 2>&1; then echo "Refusing to overwrite existing volume: $volume" >&2; exit 1; fi
done
umask 077
work="$(mktemp -d "$RUNTIME/restore.XXXXXX")"
cleanup(){ [[ "$work" == "$RUNTIME"/restore.* ]] && rm -rf -- "$work"; }
trap cleanup EXIT
openssl enc -d -aes-256-cbc -pbkdf2 -pass env:BACKUP_PASSWORD -in "$1" -out "$work/archive.tar.gz"
if tar -tzf "$work/archive.tar.gz" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then echo 'Unsafe archive paths.' >&2; exit 1; fi
mkdir "$work/unpacked"
tar -xzf "$work/archive.tar.gz" -C "$work/unpacked"
mkdir -p "$RUNTIME/fabric"
cp -a "$work/unpacked/data" "$RUNTIME/data"
cp -a "$work/unpacked/organizations" "$RUNTIME/fabric/organizations"
cp "$work/unpacked/channel.block" "$RUNTIME/fabric/channel.block"
for volume in campus-return_orderer-data campus-return_peer1-data campus-return_peer2-data; do
  docker volume create "$volume" >/dev/null
  docker run --rm --network none -v "$volume:/target" -v "$work/unpacked:/backup:ro" busybox tar -xzf "/backup/$volume.tgz" -C /target
done
echo 'Restored to an empty environment. Run scripts/start.sh to verify and start.'
