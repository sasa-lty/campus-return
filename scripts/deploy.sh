#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/env.sh"
peer_env 1
SOURCE_HASH="$(cd "$ROOT" && sha256sum main.go go.mod go.sum internal/domain/*.go internal/ledger/*.go | sha256sum | cut -d' ' -f1)"
SEQUENCE=1
if peer lifecycle chaincode querycommitted --channelID "$CHANNEL_NAME" --name "$CHAINCODE_NAME" --output json > "$RUNTIME/committed.json" 2>/dev/null; then
  if [[ -f "$RUNTIME/deployed-source.sha" && "$(<"$RUNTIME/deployed-source.sha")" == "$SOURCE_HASH" ]]; then echo 'Unchanged chaincode; existing ledger retained.'; exit 0; fi
  SEQUENCE="$(jq -r '.sequence + 1' "$RUNTIME/committed.json")"
fi
VERSION="1.$SEQUENCE"
CC="$RUNTIME/chaincode-source"
mkdir -p "$CC/internal/domain" "$CC/internal/ledger"
cp "$ROOT/go.mod" "$ROOT/go.sum" "$ROOT/main.go" "$CC/"
cp "$ROOT/internal/domain/"*.go "$CC/internal/domain/"
cp "$ROOT/internal/ledger/"*.go "$CC/internal/ledger/"
(cd "$CC" && go mod tidy && go mod vendor)
peer lifecycle chaincode package "$RUNTIME/campusreturn.tar.gz" --path "$CC" --lang golang --label "campusreturn_${SEQUENCE}_${SOURCE_HASH:0:12}"
PACKAGE_ID="$(peer lifecycle chaincode calculatepackageid "$RUNTIME/campusreturn.tar.gz")"
ORDERER=( -o localhost:17050 --ordererTLSHostnameOverride cr-orderer.example.com --tls --cafile "$ORDERER_TLS/ca.crt" )
POLICY="AND('Org1MSP.peer','Org2MSP.peer')"
for org in 1 2; do
  peer_env "$org"
  if ! peer lifecycle chaincode queryinstalled | grep -Fq "$PACKAGE_ID"; then peer lifecycle chaincode install "$RUNTIME/campusreturn.tar.gz"; fi
  peer lifecycle chaincode approveformyorg "${ORDERER[@]}" --channelID "$CHANNEL_NAME" --name "$CHAINCODE_NAME" --version "$VERSION" --sequence "$SEQUENCE" --package-id "$PACKAGE_ID" --signature-policy "$POLICY"
done
peer_env 1
peer lifecycle chaincode commit "${ORDERER[@]}" --channelID "$CHANNEL_NAME" --name "$CHAINCODE_NAME" --version "$VERSION" --sequence "$SEQUENCE" --signature-policy "$POLICY" --peerAddresses localhost:17051 --tlsRootCertFiles "$FABRIC_ROOT/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:19051 --tlsRootCertFiles "$FABRIC_ROOT/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt"
peer lifecycle chaincode querycommitted --channelID "$CHANNEL_NAME" --name "$CHAINCODE_NAME"
printf '%s\n' "$SOURCE_HASH" > "$RUNTIME/deployed-source.sha"
