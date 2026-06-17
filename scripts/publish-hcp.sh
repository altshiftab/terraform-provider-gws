#!/usr/bin/env bash
#
# Publish the artifacts in dist/ (produced by `goreleaser release --clean --skip=publish`)
# to the HCP Terraform private registry.
#
# Requires: curl, jq, and:
#   TF_TOKEN_app_terraform_io  HCP token (after `terraform login`, or an API token)
#   GPG_KEY_ID                 key-id printed by scripts/hcp-register-gpg-key.sh
#
#   export TF_TOKEN_app_terraform_io=<token>
#   export GPG_KEY_ID=<key-id>
#   ./scripts/publish-hcp.sh
#
set -euo pipefail

ORG="${HCP_ORG:-altshift}"
NAMESPACE="${HCP_NAMESPACE:-$ORG}"
NAME="${PROVIDER_NAME:-gws}"
DIST="${DIST_DIR:-dist}"
API="https://app.terraform.io/api/v2"
TOKEN="${TF_TOKEN_app_terraform_io:-}"
KEY_ID="${GPG_KEY_ID:-}"

[[ -n "$TOKEN" ]]  || { echo "error: TF_TOKEN_app_terraform_io not set" >&2; exit 1; }
[[ -n "$KEY_ID" ]] || { echo "error: GPG_KEY_ID not set (run scripts/hcp-register-gpg-key.sh)" >&2; exit 1; }
[[ -f "$DIST/metadata.json" ]] || { echo "error: $DIST/metadata.json missing — run goreleaser first" >&2; exit 1; }

VERSION="$(jq -r .version "$DIST/metadata.json")"
PROJECT="$(jq -r .project_name "$DIST/metadata.json")"
SHASUMS="$DIST/${PROJECT}_${VERSION}_SHA256SUMS"
SHASUMS_SIG="${SHASUMS}.sig"

[[ -f "$SHASUMS" ]]     || { echo "error: $SHASUMS missing" >&2; exit 1; }
[[ -f "$SHASUMS_SIG" ]] || { echo "error: $SHASUMS_SIG missing — did the signing step run?" >&2; exit 1; }

auth=(--header "Authorization: Bearer $TOKEN")
ct=(--header "Content-Type: application/vnd.api+json")

echo ">> Publishing ${NAMESPACE}/${NAME} ${VERSION} to HCP org ${ORG}"

# 1. Ensure the provider exists (422 = already created, which is fine).
prov_payload="$(jq -n --arg name "$NAME" --arg ns "$NAMESPACE" '{
  data: { type: "registry-providers", attributes: {
    name: $name, namespace: $ns, "registry-name": "private" } }
}')"
code="$(curl -sS -o /tmp/hcp_prov.json -w '%{http_code}' "${auth[@]}" "${ct[@]}" \
  -X POST --data "$prov_payload" "$API/organizations/$ORG/registry-providers")"
case "$code" in
  201) echo "   provider created" ;;
  422) echo "   provider already exists" ;;
  *)   echo "   provider create returned HTTP $code:"; jq . </tmp/hcp_prov.json; exit 1 ;;
esac

# 2. Create the version; capture the SHA256SUMS upload links.
ver_payload="$(jq -n --arg v "$VERSION" --arg k "$KEY_ID" '{
  data: { type: "registry-provider-versions", attributes: {
    version: $v, "key-id": $k, protocols: ["6.0"] } }
}')"
ver_resp="$(curl -sS "${auth[@]}" "${ct[@]}" -X POST --data "$ver_payload" \
  "$API/organizations/$ORG/registry-providers/private/$NAMESPACE/$NAME/versions")"
shasums_upload="$(echo "$ver_resp"     | jq -r '.data.links."shasums-upload" // empty')"
shasums_sig_upload="$(echo "$ver_resp" | jq -r '.data.links."shasums-sig-upload" // empty')"
if [[ -z "$shasums_upload" || -z "$shasums_sig_upload" ]]; then
  echo "error: could not create version (already exists?). Response:" >&2
  echo "$ver_resp" | jq . >&2
  exit 1
fi

# 3. Upload the signed checksums.
echo ">> Uploading SHA256SUMS + signature"
curl -sS -X PUT -T "$SHASUMS"     "$shasums_upload"
curl -sS -X PUT -T "$SHASUMS_SIG" "$shasums_sig_upload"

# 4. Register each platform, then upload its zip.
shopt -s nullglob
for zip in "$DIST/${PROJECT}_${VERSION}"_*.zip; do
  base="$(basename "$zip")"
  rest="${base#"${PROJECT}_${VERSION}_"}"; rest="${rest%.zip}"   # <os>_<arch>
  os="${rest%_*}"; arch="${rest##*_}"
  shasum="$(awk -v f="$base" '$2==f {print $1}' "$SHASUMS")"
  [[ -n "$shasum" ]] || { echo "error: no shasum for $base in SHA256SUMS" >&2; exit 1; }

  echo ">> Platform $os/$arch"
  plat_payload="$(jq -n --arg os "$os" --arg arch "$arch" --arg sha "$shasum" --arg fn "$base" '{
    data: { type: "registry-provider-version-platforms", attributes: {
      os: $os, arch: $arch, shasum: $sha, filename: $fn } }
  }')"
  plat_resp="$(curl -sS "${auth[@]}" "${ct[@]}" -X POST --data "$plat_payload" \
    "$API/organizations/$ORG/registry-providers/private/$NAMESPACE/$NAME/versions/$VERSION/platforms")"
  bin_upload="$(echo "$plat_resp" | jq -r '.data.links."provider-binary-upload" // empty')"
  if [[ -z "$bin_upload" ]]; then
    echo "error: no upload link for $os/$arch. Response:" >&2
    echo "$plat_resp" | jq . >&2
    exit 1
  fi
  curl -sS -X PUT -T "$zip" "$bin_upload"
  echo "   uploaded"
done

echo ">> Done. ${NAMESPACE}/${NAME} ${VERSION} is published to the HCP private registry."
