#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
command -v cryptogen >/dev/null
docker info >/dev/null
if [[ ! -d "$FABRIC_ROOT/organizations" ]]; then
  cryptogen generate --config="$ROOT/deployment/crypto-config.yaml" --output="$FABRIC_ROOT/organizations"
fi
cp "$ROOT/deployment/configtx.yaml" "$FABRIC_ROOT/configtx.yaml"
if [[ ! -f "$FABRIC_ROOT/channel.block" ]]; then
  FABRIC_CFG_PATH="$FABRIC_ROOT" configtxgen -profile CampusReturn -channelID "$CHANNEL_NAME" -outputBlock "$FABRIC_ROOT/channel.block"
fi
docker compose -f "$ROOT/deployment/compose.yaml" up -d
ready=0
for ((i=0;i<30;i++)); do
  if osnadmin channel list -o localhost:17053 --ca-file "$ORDERER_TLS/ca.crt" --client-cert "$ORDERER_TLS/server.crt" --client-key "$ORDERER_TLS/server.key" >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == 1 ]] || { echo "Orderer not ready" >&2; exit 1; }
if ! osnadmin channel list -c "$CHANNEL_NAME" -o localhost:17053 --ca-file "$ORDERER_TLS/ca.crt" --client-cert "$ORDERER_TLS/server.crt" --client-key "$ORDERER_TLS/server.key" | grep -q '"name"'; then
  osnadmin channel join --channelID "$CHANNEL_NAME" --config-block "$FABRIC_ROOT/channel.block" -o localhost:17053 --ca-file "$ORDERER_TLS/ca.crt" --client-cert "$ORDERER_TLS/server.crt" --client-key "$ORDERER_TLS/server.key"
fi
for org in 1 2; do
  peer_env "$org"
  ready=0
  for ((i=0;i<30;i++)); do
    if peer channel list > "$RUNTIME/peer-$org-channels.log" 2>&1; then ready=1; break; fi
    sleep 1
  done
  [[ "$ready" == 1 ]] || { echo "Peer $org not ready" >&2; exit 1; }
  if ! grep -q "$CHANNEL_NAME" "$RUNTIME/peer-$org-channels.log"; then peer channel join -b "$FABRIC_ROOT/channel.block"; fi
done
