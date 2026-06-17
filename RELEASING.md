# Releasing

This provider is distributed through the **HCP Terraform private registry** as
`app.terraform.io/altshift/gws` (HCP org `altshift`). It is not published to the
public Terraform registry, and there is no CI release pipeline — releases are cut
from a local machine.

## Prerequisites (one-time)

- `goreleaser`, `gpg`, `jq`, and `curl` installed.
- The release **signing GPG key** present in the local keyring
  (fingerprint `F3FA05768865F190FBB0EF51888CD0428F90299C`, key-id `888CD0428F90299C`,
  no passphrase). Its public half is already registered with the HCP org via
  `scripts/hcp-register-gpg-key.sh` — only re-run that if the key changes.
- An HCP token. After `terraform login`, the scripts read it from
  `~/.terraform.d/credentials.tfrc.json`.

## Cutting a release

```sh
# 1. Commit your changes, then tag. Tags must be annotated (-m).
git commit -am "..."
git tag -a vX.Y.Z -m vX.Y.Z

# 2. Build + sign artifacts into dist/ (build env requires GOEXPERIMENT=jsonv2,
#    which .goreleaser.yml sets).
GPG_FINGERPRINT=F3FA05768865F190FBB0EF51888CD0428F90299C \
  goreleaser release --clean --skip=publish

# 3. Publish dist/ to the HCP private registry.
export TF_TOKEN_app_terraform_io="$(jq -r '.credentials["app.terraform.io"].token' ~/.terraform.d/credentials.tfrc.json)"
export GPG_KEY_ID=888CD0428F90299C
./scripts/publish-hcp.sh

# 4. (optional) Push the commit and tag.
git push origin main && git push origin vX.Y.Z
```

Consumers then get the new version via their `version` constraint on the next
`terraform init -upgrade`.

## Notes

- **Always bump the tag.** HCP rejects re-publishing a version that already exists.
- `scripts/publish-hcp.sh` creates the provider on first run and adds a version on
  subsequent runs. Override `HCP_ORG` / `PROVIDER_NAME` / `GPG_KEY_ID` via env if needed.
- **Local development:** to test the provider against `altshift_infrastructure`
  without publishing, re-enable the (currently commented) `dev_overrides` block in
  `~/.terraformrc`, keyed on `app.terraform.io/altshift/gws`.
- **Changing the source/namespace** (e.g. moving registries) requires consumers to run
  `terraform state replace-provider <old-fqn> <new-fqn>`, because existing resources are
  bound to the old provider address in state.
