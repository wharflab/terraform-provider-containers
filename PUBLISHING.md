# Registry Publishing

## One-time setup

Prerequisites: `brew install gnupg gh jq` and `gh auth login` with org admin scope.

```bash
./scripts/setup-publishing.sh
```

The script automates:

1. **GPG key generation** — RSA 4096-bit, no passphrase (CI-only key)
2. **GitHub secrets** — sets `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` via `gh`
3. **Terraform Registry** — copies public key to clipboard and opens the publish page
4. **OpenTofu Registry** — creates the provider submission issue and opens the signing key form

Browser interactions required (no API exists for these):
- Terraform Registry: OAuth sign-in + select repo + paste GPG key
- OpenTofu Registry: paste GPG public key in the signing key issue form

## How the registries work

**Terraform Registry** — connects directly to the GitHub repo. Once linked, it automatically discovers new GitHub Releases tagged with semver (`v0.1.0`).

**OpenTofu Registry** — provider is submitted via [issue template](https://github.com/opentofu/registry/issues/new?template=provider.yml), GPG key via a [separate issue](https://github.com/opentofu/registry/issues/new?template=provider_key.yml). After approval, the registry automation indexes new releases automatically (usually within 30 minutes).

## Releasing a version

```bash
git tag v0.1.0 && git push origin v0.1.0
```

The release workflow runs tests, builds for 6 platform targets, signs checksums with GPG, publishes a GitHub Release, and both registries pick it up automatically.

## Artifacts produced per release

| File | Purpose |
|------|---------|
| `terraform-provider-containers_VERSION_OS_ARCH.zip` | Provider binary (6 targets) |
| `terraform-provider-containers_VERSION_SHA256SUMS` | Checksums |
| `terraform-provider-containers_VERSION_SHA256SUMS.sig` | GPG signature |
| `terraform-provider-containers_VERSION_manifest.json` | Registry protocol metadata |

## Verify

```bash
# Check registries
open "https://registry.terraform.io/providers/wharflab/containers"
open "https://search.opentofu.org/provider/wharflab/containers"

# Test install
mkdir /tmp/tf-test && cd /tmp/tf-test
cat > main.tf <<'HCL'
terraform {
  required_providers {
    containers = {
      source  = "wharflab/containers"
      version = "~> 0.1"
    }
  }
}
HCL
terraform init
```

## Key rotation

```bash
# Generate new key, update GitHub secrets
./scripts/setup-publishing.sh

# Terraform Registry: re-upload public key via web UI
# OpenTofu Registry: submit new key via issue template
#   https://github.com/opentofu/registry/issues/new?template=provider_key.yml
```
