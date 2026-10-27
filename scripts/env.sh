#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME="${CAMPUS_RETURN_RUNTIME:-$HOME/.local/share/campus-return}"
export FABRIC_ROOT="$RUNTIME/fabric"
export DATA_DIR="$RUNTIME/data"
export FRONTEND_DIR="$ROOT/frontend/dist"
export APP_ADDR="${APP_ADDR:-127.0.0.1:8088}"
export CHANNEL_NAME=returnchannel
export CHAINCODE_NAME=campusreturn
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
# 支持标准安装位置；保留已有本机配置的兼容回退。
if [[ -z "${FABRIC_SAMPLES_PATH:-}" ]]; then
  if [[ -d "$HOME/fabric-samples/bin" ]]; then FABRIC_SAMPLES_PATH="$HOME/fabric-samples";
  else FABRIC_SAMPLES_PATH="$HOME/fabric-lab7-wsl-demo-verify/fabric-resources/fabric-samples"; fi
fi
export PATH="$RUNTIME/bin:$FABRIC_SAMPLES_PATH/bin:$PATH"
export FABRIC_CFG_PATH="$FABRIC_SAMPLES_PATH/config"
umask 077
mkdir -p "$RUNTIME" "$DATA_DIR" "$RUNTIME/bin" "$FABRIC_ROOT"
if [[ ! -f "$RUNTIME/secrets.env" ]]; then
  printf 'export APP_ENCRYPTION_KEY=%s\nexport ADMIN_PASSWORD=%s\n' "$(openssl rand -hex 32)" "Cr-$(openssl rand -hex 14)" > "$RUNTIME/secrets.env"
fi
source "$RUNTIME/secrets.env"
ORDERER_TLS="$FABRIC_ROOT/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls"
peer_env() {
  local number="$1"; local org="org${number}.example.com"
  export CORE_PEER_TLS_ENABLED=true
  export CORE_PEER_LOCALMSPID="Org${number}MSP"
  export CORE_PEER_MSPCONFIGPATH="$FABRIC_ROOT/organizations/peerOrganizations/$org/users/Admin@$org/msp"
  export CORE_PEER_TLS_ROOTCERT_FILE="$FABRIC_ROOT/organizations/peerOrganizations/$org/peers/peer0.$org/tls/ca.crt"
  if [[ "$number" == 1 ]]; then export CORE_PEER_ADDRESS=localhost:17051; else export CORE_PEER_ADDRESS=localhost:19051; fi
}
