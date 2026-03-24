#!/usr/bin/env bash
#
# One-time setup for publishing terraform-provider-containers to
# the Terraform Registry and OpenTofu Registry.
#
# Prerequisites:
#   brew install gnupg gh jq
#   gh auth login            (needs admin:org scope for org secrets)
#
# What this script does:
#   1. Generates a dedicated GPG signing key (no passphrase, CI-only)
#   2. Exports and stores the private key + fingerprint as GitHub org secrets
#   3. Saves the public key for registry submission
#   4. Opens the Terraform Registry publish page (one click remaining)
#   5. Opens a pre-filled OpenTofu Registry submission issue
#
# Usage:
#   ./scripts/setup-publishing.sh
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — edit these if the org/repo/email ever changes
# ---------------------------------------------------------------------------
ORG="wharflab"
REPO="terraform-provider-containers"
PROVIDER_NAME="containers"
KEY_EMAIL="release@wharflab.dev"
KEY_REAL_NAME="WharfLab Release Signing"
OPENTOFU_REGISTRY_REPO="opentofu/registry"  # for issue submission

# ---------------------------------------------------------------------------
# Preflight checks
# ---------------------------------------------------------------------------
for cmd in gpg gh jq base64; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not found. Install it first." >&2
    exit 1
  fi
done

echo "Checking gh auth status..."
if ! gh auth status &>/dev/null; then
  echo "ERROR: Not authenticated with gh. Run: gh auth login" >&2
  exit 1
fi

# Verify we can manage secrets for the org
if ! gh api "orgs/${ORG}" --jq .login &>/dev/null; then
  echo "WARNING: Cannot access org ${ORG}. Secrets will be set at repo level instead."
  SECRET_SCOPE="--repo ${ORG}/${REPO}"
else
  SECRET_SCOPE="--org ${ORG} --visibility selected --repos ${REPO}"
fi

# ---------------------------------------------------------------------------
# Step 1: Generate GPG key
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 1: Generating GPG signing key ==="

# Check if key already exists
if gpg --list-keys "${KEY_EMAIL}" &>/dev/null; then
  FINGERPRINT=$(gpg --list-keys --with-colons "${KEY_EMAIL}" | awk -F: '/^fpr/ {print $10; exit}')
  echo "GPG key already exists for ${KEY_EMAIL}: ${FINGERPRINT}"
  read -rp "Use existing key? [Y/n] " use_existing
  if [[ "${use_existing:-Y}" =~ ^[Nn] ]]; then
    echo "Aborting. Delete the existing key first if you want a new one:"
    echo "  gpg --delete-secret-and-public-key ${FINGERPRINT}"
    exit 1
  fi
else
  GPG_PARAMS=$(mktemp)
  cat > "${GPG_PARAMS}" <<GPGEOF
%no-protection
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: ${KEY_REAL_NAME}
Name-Email: ${KEY_EMAIL}
Expire-Date: 0
%commit
GPGEOF

  gpg --batch --gen-key "${GPG_PARAMS}"
  rm -f "${GPG_PARAMS}"

  FINGERPRINT=$(gpg --list-keys --with-colons "${KEY_EMAIL}" | awk -F: '/^fpr/ {print $10; exit}')
  echo "Generated GPG key: ${FINGERPRINT}"
fi

# ---------------------------------------------------------------------------
# Step 2: Export keys and set GitHub secrets
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 2: Setting GitHub Actions secrets ==="

GPG_PRIVATE_KEY=$(gpg --armor --export-secret-keys "${FINGERPRINT}")
GPG_PUBLIC_KEY=$(gpg --armor --export "${FINGERPRINT}")

# shellcheck disable=SC2086
echo "${GPG_PRIVATE_KEY}" | gh secret set GPG_PRIVATE_KEY ${SECRET_SCOPE}
echo "  ✓ GPG_PRIVATE_KEY set"

# shellcheck disable=SC2086
echo "" | gh secret set GPG_PASSPHRASE ${SECRET_SCOPE}
echo "  ✓ GPG_PASSPHRASE set (empty — key has no passphrase)"

# ---------------------------------------------------------------------------
# Step 3: Save public key locally
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 3: Saving public key ==="

PUBKEY_FILE="$(pwd)/signing-key.pub.asc"
echo "${GPG_PUBLIC_KEY}" > "${PUBKEY_FILE}"
echo "  ✓ Public key saved to ${PUBKEY_FILE}"
echo "  ✓ Fingerprint: ${FINGERPRINT}"

# ---------------------------------------------------------------------------
# Step 4: Terraform Registry onboarding
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 4: Terraform Registry ==="
echo ""
echo "The Terraform Registry requires a one-time OAuth sign-in via browser."
echo "The publish page will open now. Steps once it loads:"
echo ""
echo "  1. Sign in with the GitHub account that owns '${ORG}'"
echo "  2. Click 'Publish' → 'Provider'"
echo "  3. Select repository: ${ORG}/${REPO}"
echo "  4. Paste the GPG public key when prompted"
echo ""

# Copy public key to clipboard if possible
if command -v pbcopy &>/dev/null; then
  echo "${GPG_PUBLIC_KEY}" | pbcopy
  echo "  ✓ Public key copied to clipboard (ready to paste)"
elif command -v xclip &>/dev/null; then
  echo "${GPG_PUBLIC_KEY}" | xclip -selection clipboard
  echo "  ✓ Public key copied to clipboard (ready to paste)"
else
  echo "  (could not copy to clipboard — paste from ${PUBKEY_FILE})"
fi

read -rp "Press Enter to open the Terraform Registry publish page..."
if command -v open &>/dev/null; then
  open "https://registry.terraform.io/publish/provider"
elif command -v xdg-open &>/dev/null; then
  xdg-open "https://registry.terraform.io/publish/provider"
else
  echo "  Open manually: https://registry.terraform.io/publish/provider"
fi

read -rp "Press Enter once you've completed the Terraform Registry setup..."

# ---------------------------------------------------------------------------
# Step 5: OpenTofu Registry — submit via issue template
# ---------------------------------------------------------------------------
echo ""
echo "=== Step 5: OpenTofu Registry ==="
echo ""
echo "OpenTofu accepts new providers via a GitHub issue (not a PR)."
echo "Two issues are needed:"
echo "  1. Submit the provider repository"
echo "  2. Submit the GPG signing key"
echo ""

# Build the pre-filled issue URLs.
# The OpenTofu issue template for providers uses a "repository" field.
PROVIDER_ISSUE_URL="https://github.com/opentofu/registry/issues/new?template=provider.yml&title=$(jq -rn --arg t "New Provider: ${ORG}/${PROVIDER_NAME}" '$t | @uri')&repository=$(jq -rn --arg r "${ORG}/${REPO}" '$r | @uri')"

KEY_ISSUE_URL="https://github.com/opentofu/registry/issues/new?template=provider_key.yml&title=$(jq -rn --arg t "Signing Key: ${ORG}/${PROVIDER_NAME}" '$t | @uri')&namespace=$(jq -rn --arg n "${ORG}" '$n | @uri')"

# Submit provider issue via gh CLI
echo "  Creating provider submission issue..."
PROVIDER_ISSUE=$(gh issue create \
  --repo opentofu/registry \
  --title "New Provider: ${ORG}/${PROVIDER_NAME}" \
  --body "### Provider Repository
${ORG}/${REPO}

### Description
Terraform/OpenTofu provider for remote container builds and OCI artifact production using cloud-native build backends (AWS CodeBuild)." 2>&1) || true

if [[ "${PROVIDER_ISSUE}" == *"already exists"* ]] || [[ -z "${PROVIDER_ISSUE}" ]]; then
  echo "  Could not create issue automatically. Opening browser with pre-filled form..."
  if command -v open &>/dev/null; then
    open "${PROVIDER_ISSUE_URL}"
  elif command -v xdg-open &>/dev/null; then
    xdg-open "${PROVIDER_ISSUE_URL}"
  else
    echo "  Open manually: ${PROVIDER_ISSUE_URL}"
  fi
else
  echo "  ✓ Provider issue: ${PROVIDER_ISSUE}"
fi

# Submit signing key issue
echo ""
echo "  Now submitting the GPG signing key..."
echo "  The key submission form requires pasting the ASCII-armored public key."

if command -v pbcopy &>/dev/null; then
  echo "${GPG_PUBLIC_KEY}" | pbcopy
  echo "  ✓ Public key copied to clipboard"
elif command -v xclip &>/dev/null; then
  echo "${GPG_PUBLIC_KEY}" | xclip -selection clipboard
  echo "  ✓ Public key copied to clipboard"
fi

echo ""
read -rp "Press Enter to open the signing key submission form..."
if command -v open &>/dev/null; then
  open "${KEY_ISSUE_URL}"
elif command -v xdg-open &>/dev/null; then
  xdg-open "${KEY_ISSUE_URL}"
else
  echo "  Open manually: ${KEY_ISSUE_URL}"
fi

echo ""
echo "  In the form, paste the GPG public key from your clipboard."
echo "  The registry automation will index releases within ~30 minutes"
echo "  after both issues are processed."
read -rp "Press Enter once you've submitted the signing key issue..."

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "==========================================="
echo "  Publishing setup complete"
echo "==========================================="
echo ""
echo "  GPG fingerprint : ${FINGERPRINT}"
echo "  Public key file : ${PUBKEY_FILE}"
echo "  GitHub secrets  : GPG_PRIVATE_KEY, GPG_PASSPHRASE"
echo ""
echo "  To publish the first release:"
echo "    git tag v0.1.0 && git push origin v0.1.0"
echo ""
echo "  Verify after release:"
echo "    https://registry.terraform.io/providers/${ORG}/${PROVIDER_NAME}"
echo "    https://search.opentofu.org/provider/${ORG}/${PROVIDER_NAME}"
echo ""
