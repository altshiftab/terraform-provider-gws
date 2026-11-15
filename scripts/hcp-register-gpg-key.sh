#!/usr/bin/env bash
#
# One-time: register the provider's signing GPG *public* key with the HCP org.
# Prints the resulting key-id, which you then pass as GPG_KEY_ID to publish-hcp.sh.
#
# Requires: gpg, curl, jq, and an HCP token in TF_TOKEN_app_terraform_io
# (set it after `terraform login`, or use an org/team API token).
#
#   TF_TOKEN_app_terraform_io=<token> ./scripts/hcp-register-gpg-key.sh
#
set -euo pipefail

ORG="${HCP_ORG:-altshift}"
# By fingerprint: the keyring also holds the retired key's public half, under the same email.
KEY="${GPG_FINGERPRINT:-ECC40A6D6CA5C7789B933208D06826DF3F80C139}"
TOKEN="${TF_TOKEN_app_terraform_io:-}"

[[ -n "$TOKEN" ]] || { echo "error: TF_TOKEN_app_terraform_io is not set" >&2; exit 1; }

ascii_armor="$(gpg --armor --export "$KEY")"
[[ -n "$ascii_armor" ]] || { echo "error: no public key found for $KEY" >&2; exit 1; }

payload="$(jq -n --arg ns "$ORG" --arg armor "$ascii_armor" '{
  data: {
    type: "gpg-keys",
    attributes: { namespace: $ns, "ascii-armor": $armor }
  }
}')"

echo ">> Registering GPG key for namespace '$ORG'..."
resp="$(curl -sS \
  --header "Authorization: Bearer $TOKEN" \
  --header "Content-Type: application/vnd.api+json" \
  --request POST \
  --data "$payload" \
  https://app.terraform.io/api/registry/private/v2/gpg-keys)"

echo "$resp" | jq .

key_id="$(echo "$resp" | jq -r '.data.attributes."key-id" // empty')"
if [[ -n "$key_id" ]]; then
  echo
  echo ">> Registered. key-id = $key_id"
  echo ">> Export it before publishing:  export GPG_KEY_ID=$key_id"
fi
