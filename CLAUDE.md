# CLAUDE.md — terraform-provider-containers

## What this is

Terraform/OpenTofu provider for remote container builds and OCI artifact production.
Provider TypeName is `containers` — resources are prefixed `containers_` (not `wharflab_`).
Source address: `wharflab/containers`. Apache 2.0 license.

## Build and test

```bash
make              # build the binary
make test         # unit tests
make testacc      # acceptance tests (needs AWS creds + TF_ACC=1)
make vet          # go vet
make lint         # golangci-lint (install separately)
```

Single command to verify everything: `go build ./... && go vet ./... && go test ./... -count=1`

## Architecture

```
main.go                          → provider entry point
internal/
  provider/
    provider.go                  → AWS credential chain, resource registration
    aws_codebuild_builder_resource.go → containers_aws_codebuild_builder (CRUD for CodeBuild project)
    build_resource.go            → containers_build (build execution + digest capture)
  backend/
    codebuild.go                 → CodeBuild API calls, buildspec generation, S3 upload/read
  buildcontext/
    context.go                   → context types
    local.go                     → local directory preparation (hash + archive)
    archive.go                   → deterministic tar.gz creation
    dockerignore.go              → .dockerignore parsing via moby/patternmatcher
    hash.go                      → SHA256 content hashing for change detection
  oci/
    reference.go                 → OCI reference parsing, ECR region extraction
```

## Key design decisions

- **Provider TypeName `containers`**, not `wharflab` — keeps resource names short (`containers_build` not `wharflab_build`).
- **All resources in `internal/provider/` package** — one file per resource, no sub-packages for resources.
- **`ProviderData` struct** passed to resources via `Configure()` — carries `aws.Config`.
- **Build resource uses `ModifyPlan`** to compute context hash during planning — Terraform detects drift without apply.
- **Buildspec is inline** (passed via `StartBuild` override) — each build gets its own buildspec, project is just compute config.
- **Build metadata captured via S3** — buildspec writes digest JSON to `s3://bucket/prefix/results/{id}.json`, provider reads after build completes.
- **Context archives are deterministic** — sorted entries, epoch timestamps, normalized ownership, .git excluded.

## Build execution flow

1. Plan: `ModifyPlan` hashes local directory → `context_hash` computed → Terraform detects change
2. Apply: archive created → uploaded to S3 → CodeBuild `StartBuild` with inline buildspec
3. Buildspec: ECR login → `docker buildx build --push --metadata-file` → result JSON → `aws s3 cp` to result key
4. Provider: poll `BatchGetBuilds` → read result JSON from S3 → store digest in state

## Conventions

- **Automate everything.** If it takes more than one manual step, script it. See `scripts/setup-publishing.sh` as the model — GPG key generation through OpenTofu Registry PR, one script.
- **No interactive prompts in CI.** GPG keys have no passphrase. Scripts use `--batch` flags.
- **Tests go next to code.** `foo.go` → `foo_test.go` in same package. No separate test directories.
- **Errors must be actionable.** Include the build ID, S3 path, or log URL — never just "build failed".
- **Digest-first outputs.** `image_digest` and `image_ref` (with digest) are the primary outputs, not tags.
- **terraform-plugin-framework** (not the older SDK). Use typed schemas, plan modifiers, `diag.Diagnostics`.
- **AWS SDK v2** throughout. No v1 imports.

## Adding a new resource

1. Create `internal/provider/{name}_resource.go` with the resource struct, schema, and CRUD methods.
2. Register it in `provider.go` → `Resources()` slice.
3. Add docs at `docs/resources/{name}.md` and example at `examples/resources/containers_{name}/resource.tf`.
4. Add tests. Unit tests for logic, acceptance tests for AWS integration.

## Adding a new backend (future)

Follow the pattern from `internal/backend/codebuild.go`:
- Expose `StartBuild`, `WaitForBuild`, `ReadResultMetadata` functions
- The build resource resolves which backend to call based on builder type
- Backend-specific builder resources go in `internal/provider/` as separate files

## Release process

```bash
git tag v0.1.0 && git push origin v0.1.0
```

That's it. GitHub Actions builds 6 platform binaries, signs checksums, publishes the GitHub Release. Both registries pick it up automatically. See `PUBLISHING.md` and `scripts/setup-publishing.sh` for one-time registry onboarding.

## Design reference

Full system design: `docs/system-design-mvp.md`. Current implementation covers Phase 1 (MVP):
- `containers_aws_codebuild_builder` + `containers_build`
- Local directory and S3 context sources
- Dockerfile builds, ECR publication, digest outputs

Phase 2 adds `containers_bake_build` and `containers_compose_build`.
Phase 3 adds multi-platform and cache policies.
Phase 4 adds provenance, SBOM, and signing.
