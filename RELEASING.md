# Releasing

This provider is distributed through the **HCP Terraform private registry** as
`app.terraform.io/altshift/gws` (HCP org `altshift`). It is not published to the
public Terraform registry, and there is no CI release pipeline — releases are cut
from a local machine.

## Prerequisites (one-time)

- `goreleaser`, `gpg`, `jq`, and `curl` installed.
- The release **signing GPG key** in the local keyring (fingerprint `ECC40A6D6CA5C7789B933208D06826DF3F80C139`, key-id `D06826DF3F80C139`). It is RSA 2048 held in capa's TPM (moved there with `keytotpm`), so it signs only on capa, asks for its PIN, and has no other copy. That keyring's `gpg.conf` sets `digest-algo SHA256`, because the TPM cannot sign SHA-512. Its public half is registered with the HCP org via `scripts/hcp-register-gpg-key.sh`. If the TPM is lost, make a new key the same way and register it; every version released before 2026-11-15 was signed with the earlier key `888CD0428F90299C`, which stays registered so those versions still verify.
- An HCP token. `terraform login` stores it sealed to capa's TPM through the `tpm` credentials helper (`~/.terraform.d/plugins/terraform-credentials-tpm`, named in `~/.terraformrc`), and step 3 reads it back from there.

## Cutting a release

```sh
# 1. Commit your changes, then tag. Tags must be annotated (-m).
git commit -am "..."
git tag -a vX.Y.Z -m vX.Y.Z

# 2. Build + sign artifacts into dist/.
GPG_FINGERPRINT=ECC40A6D6CA5C7789B933208D06826DF3F80C139 \
  goreleaser release --clean --skip=publish

# 3. Publish dist/ to the HCP private registry.
export TF_TOKEN_app_terraform_io="$(~/.terraform.d/plugins/terraform-credentials-tpm get app.terraform.io | jq -r .token)"
export GPG_KEY_ID=D06826DF3F80C139
./scripts/publish-hcp.sh

# 4. Mirror the archives for the drift check, which reads the provider from a
#    bucket rather than holding a credential for the registry above.
gcloud storage cp dist/terraform-provider-gws_X.Y.Z_*.zip \
  gs://altshift-terraform-provider-mirror/app.terraform.io/altshift/gws/ \
  --project=altshift-main

# 5. (optional) Push the commit and tag.
git push origin main && git push origin vX.Y.Z
```

Consumers then get the new version via their `version` constraint on the next
`terraform init -upgrade`.

## Notes

- **Always bump the tag.** HCP rejects re-publishing a version that already exists.
- **Do not skip the mirror.** `altshift_infrastructure` runs a daily drift check as an
  account that deliberately holds no credential for the HCP registry: this organisation's
  plan entitles no teams, so the narrowest token it can issue could also publish here.
  The check reads the provider from the bucket instead, through a `filesystem_mirror` that
  has no `direct` fallback for it. A version that is published but not mirrored is a
  version that check cannot install. Leave older versions in place; a rollback needs them.
- `scripts/publish-hcp.sh` creates the provider on first run and adds a version on
  subsequent runs. Override `HCP_ORG` / `PROVIDER_NAME` / `GPG_KEY_ID` via env if needed.
- **Local development:** to test the provider against `altshift_infrastructure`
  without publishing, re-enable the (currently commented) `dev_overrides` block in
  `~/.terraformrc`, keyed on `app.terraform.io/altshift/gws`.
- **Changing the source/namespace** (e.g. moving registries) requires consumers to run
  `terraform state replace-provider <old-fqn> <new-fqn>`, because existing resources are
  bound to the old provider address in state.
