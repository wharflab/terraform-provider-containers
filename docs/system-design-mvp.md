# WharfLab Terraform Container Provider — System Design

## Status

Draft v0.2

## Purpose

This document defines the system design for a new WharfLab Terraform/OpenTofu provider focused on **remote container builds**, **OCI artifact publication**, and **cloud-native build backends**.

The provider is intended to solve a gap not cleanly addressed by existing Docker-oriented Terraform providers: users increasingly want Terraform-managed **remote builders**, **cloud execution backends**, **immutable image outputs**, and eventually **supply-chain metadata** such as provenance and SBOMs. Existing Docker providers are historically centered on managing Docker daemon objects and only secondarily on modern build pipelines.

This design is written to be detailed enough for direct handoff to an implementation engineer.

---

## 1. Goals

### Primary goals

1. Provide a Terraform/OpenTofu-native way to run **remote container builds**.
2. Support **cloud builder backends**, with **AWS CodeBuild** as MVP and room for future **Google Cloud Build** and **Azure ACR Tasks / equivalents**.
3. Produce **immutable OCI outputs** suitable for immediate handoff to infrastructure resources such as AWS Lambda, ECS, Kubernetes, Cloud Run, etc.
4. Support **existing container authoring ecosystems** such as Dockerfile/Containerfile, Docker Bake, and optionally Compose-based build definitions.
5. Preserve **Terraform/OpenTofu ergonomics**: typed schema, plan/apply lifecycle, deterministic inputs, documented outputs, stable resource behavior.
6. Publish as a real provider consumable by both **Terraform** and **OpenTofu**, not as a GitHub-only plugin.

### Secondary goals

1. Build a strong foundation for future:
   - multi-platform builds
   - build cache import/export
   - OCI attestations
   - SBOM generation
   - image signing / promotion
   - registry-native artifact relationships
2. Keep the naming credible and standards-grounded, especially around OCI.
3. Avoid over-abstraction that hides meaningful cloud/backend differences.

### Non-goals

1. Do not become a full replacement for Docker Engine lifecycle providers.
2. Do not manage long-lived local daemon objects such as local containers, networks, or volumes.
3. Do not attempt to fully reimplement Docker Compose runtime semantics.
4. Do not own general CI/CD orchestration outside container artifact production.

---

## 2. Product positioning

### Problem statement

Current Terraform Docker integrations are strongest when talking to a Docker-compatible engine and managing Docker-native runtime objects. They are less naturally aligned with modern needs such as:

- remote isolated builds
- cloud-native builders
- build context upload and normalization
- digest-first outputs
- provenance / SBOM / attestations
- module-friendly cloud auth patterns
- portable build execution across workstations and CI

### Positioning statement

**WharfLab Containers Provider** is a Terraform/OpenTofu provider for **container artifact pipelines and remote builders**, not for local Docker runtime object management.

### Market position relative to existing Docker providers

The provider should not market itself as “a better Docker provider.”
Instead, it should market itself as:

- **Terraform-native remote build provider**
- **OCI artifact production layer**
- **Cloud-native builder backend abstraction**
- **Buildx/Bake/Compose-aware execution backend**

That is a different product category from a provider centered on Docker Engine API object lifecycle.

---

## 3. Design principles

1. **One provider, multiple backend-specific resources.**
   Avoid fragmenting into one provider per cloud. Also avoid a single giant generic resource with dozens of conditional cloud-specific fields.

2. **Concrete resources should map to real backend concepts.**
   AWS CodeBuild should be represented by an AWS-specific builder resource, GCP Cloud Build by a GCP-specific builder resource, etc.

3. **Cross-backend abstraction belongs in shared execution resources and modules.**
   Shared resources such as `build` should consume backend-specific builder IDs.

4. **OCI terminology should be used only where OCI actually defines the concept.**
   OCI is authoritative for artifacts, manifests, indexes, descriptors, and related distribution concepts. It is not authoritative for Dockerfile/Containerfile or build context.

5. **Build input semantics must be deterministic.**
   Context packaging, ignore rules, input hashing, Dockerfile resolution, and remote upload behavior must be explicit and reproducible.

6. **Digest-first outputs.**
   The provider should privilege immutable digests and OCI references over mutable tags.

7. **Remote execution by API, not by SSH-first daemon sessions.**
   The control plane should be backend APIs and object storage, not brittle interactive daemon connectivity.

8. **Native Terraform/OpenTofu UX first, external-file interoperability second.**
   Bake and Compose interop should be supported as inputs, but the typed Terraform schema remains the canonical API.

---

## 4. Naming model and trusted terminology

## 4.1 Key naming decision

The provider should use a hybrid terminology model:

- **Build-side terms** from Docker/BuildKit practice
- **Artifact-side terms** from OCI

This avoids falsely implying that OCI standardizes concepts such as `Dockerfile` or build context.

## 4.2 Recommended core vocabulary

### Build-side

- **Builder** — a remote build backend or execution target
- **Build Definition** — Dockerfile/Containerfile or equivalent build instructions
- **Build Context** — the set of files or remote source available to the build
- **Build Input** — additional contexts, secrets, SSH access, remote Git, tarballs, etc.
- **Build** — an execution request producing one or more OCI outputs

### OCI-side

- **Image** — OCI image output
- **Image Index** — multi-platform OCI output
- **Artifact** — OCI-distributed non-image or auxiliary output
- **Descriptor** — OCI content reference
- **Referrer** — OCI-associated metadata attached to a subject image/artifact
- **Registry Target** — publication destination for OCI-distributed outputs

## 4.3 Naming rules

- Do **not** invent fake OCI names for Dockerfile or Containerfile.
- Do **not** use “container” where “image” is the real output.
- Prefer “build” over “job” in user-facing resources unless a backend makes “job” a concrete API object.
- Prefer “registry target” or “publication target” over daemon-centric naming.

---

## 5. High-level architecture

The provider has four conceptual layers:

1. **Definition layer**
   - Dockerfile / Containerfile
   - Bake file
   - Compose-derived build sections
   - typed Terraform build blocks

2. **Input normalization layer**
   - context discovery
   - ignore processing
   - context hashing
   - tarball packaging or remote reference capture
   - target expansion from Bake/Compose

3. **Execution backend layer**
   - AWS CodeBuild backend (MVP)
   - future: Google Cloud Build backend
   - future: Azure ACR Tasks / remote build backend
   - future: generic remote BuildKit backend

4. **Output and publication layer**
   - OCI image and image index outputs
   - digest capture
   - tag publication
   - future provenance/SBOM/attestation outputs
   - Terraform/OpenTofu outputs for downstream infra

### High-level data flow

```text
build definition + build context + provider config
  -> normalization and hashing
  -> backend submission
  -> remote build execution
  -> image/index publication to registry
  -> digest and metadata capture
  -> Terraform/OpenTofu state and outputs
```

---

## 6. Provider scope

## 6.1 In-scope

- Remote builder resources
- Build execution resources
- Context packaging / upload
- OCI registry publication targets
- Digest and artifact metadata outputs
- Bake interop
- Limited Compose build interop
- Terraform and OpenTofu publication

## 6.2 Out-of-scope for MVP

- Local Docker container lifecycle resources
- Swarm / Kubernetes runtime management
- Full Compose orchestration semantics
- Rich UI or build dashboard
- Image signing and attestations
- Full generic policy engine
- Arbitrary CI workflow orchestration

---

## 7. Backend strategy

## 7.1 Why one provider rather than one provider per cloud

The domain boundary is coherent: **remote container build systems and OCI artifact production**.
This supports a single provider with shared logic for:

- context normalization
- Bake/Compose parsing
- digest handling
- registry publication models
- future provenance and artifact features

Cloud differences should be modeled via backend-specific resources rather than separate providers.

## 7.2 Why not one giant cross-cloud builder resource

A single `builder { cloud = ... }` resource with many cloud-specific branches becomes awkward in Terraform/OpenTofu:

- validation complexity increases
- documentation becomes harder to navigate
- schema evolution becomes brittle
- backend semantics become hidden
- users lose clarity about which arguments matter for which backend

The better design is backend-specific builder resources plus shared build resources.

## 7.3 Backend resource model

Recommended resource families:

- `wharflab_aws_codebuild_builder`
- `wharflab_gcp_cloudbuild_builder` (future)
- `wharflab_azure_acr_task_builder` (future)
- `wharflab_build`
- `wharflab_bake_build`
- `wharflab_compose_build` (limited scope)

Optional future resources:

- `wharflab_registry_target`
- `wharflab_cache_policy`
- `wharflab_attestation_policy`
- `wharflab_build_context`

---

## 8. MVP backend: AWS CodeBuild

## 8.1 Why CodeBuild first

AWS CodeBuild fits the MVP especially well because it can act as a remote isolated build worker and integrates naturally with:

- ECR
- IAM
- S3 for context storage
- CloudWatch Logs
- VPC networking if needed

This aligns well with a Terraform provider whose control plane is API-driven rather than daemon-driven.

## 8.2 Conceptual execution flow

1. Terraform/OpenTofu config defines a `wharflab_aws_codebuild_builder`.
2. `wharflab_build` normalizes the build inputs.
3. Provider packages the context or identifies a remote source.
4. Provider uploads context archive to S3 or references an existing object.
5. Provider submits a CodeBuild build execution.
6. CodeBuild executes the remote build using BuildKit/buildx-compatible tooling inside the build environment.
7. Build publishes image(s) to ECR or another registry.
8. Provider retrieves immutable digest(s), indexes, and metadata.
9. State stores outputs for downstream Terraform/OpenTofu resources.

## 8.3 AWS-specific builder resource responsibilities

`wharflab_aws_codebuild_builder` should manage or reference:

- CodeBuild project identity
- environment image
- compute type
- privileged mode / container build capability
- service role ARN
- S3 bucket/prefix for context uploads
- CloudWatch logging options
- VPC settings
- concurrency / queueing settings if applicable
- default registry auth behavior
- default build engine image/tooling version

### Example shape

```hcl
resource "wharflab_aws_codebuild_builder" "main" {
  name              = "main"
  project_name      = "wharflab-container-builder"
  service_role_arn  = aws_iam_role.codebuild.arn
  artifact_bucket   = aws_s3_bucket.build_contexts.bucket
  region            = "eu-west-1"

  environment {
    image        = "aws/codebuild/standard:7.0"
    compute_type = "BUILD_GENERAL1_MEDIUM"
    privileged   = true
  }

  logging {
    cloudwatch_enabled = true
    log_group_name     = "/aws/codebuild/wharflab-container-builder"
  }
}
```

## 8.4 Build execution contract

The provider should define and own a stable internal contract for the remote worker. That contract should include:

- where the build context archive is located
- which Dockerfile/Containerfile path to use
- platform set
- target stage
- tags and publication target
- cache import/export settings
- build args
- secret/SSH declarations
- Bake target or Compose service mapping when relevant
- expected outputs to capture

The engineer should treat this worker contract as a versioned internal API, because it will likely need to remain stable across provider versions and multiple backends.

---

## 9. Future backends

## 9.1 Google Cloud Build

Google Cloud Build is a good future backend because it offers a managed build service with strong API support. The provider should eventually model it with its own builder resource rather than forcing it into AWS-shaped abstractions.

## 9.2 Azure ACR Tasks / Azure remote container build

Azure’s most natural container-focused backend is more registry-centric than generic-worker-centric. It should therefore be modeled explicitly as an Azure-specific builder target and not forced into generic worker semantics unless the abstraction truly fits.

## 9.3 Generic remote BuildKit backend

A future generic remote BuildKit backend is possible, but should not be an MVP requirement. It would likely require its own auth, endpoint, cache, and capability model.

---

## 10. Resource model

## 10.1 Core resources

### 10.1.1 `wharflab_aws_codebuild_builder`

Represents a CodeBuild-backed remote builder configuration.

**Key inputs**
- name
- project_name or create/manage flag
- service_role_arn
- region
- environment image
- compute type
- privileged mode
- artifact bucket/prefix
- logging config
- VPC config
- default registry auth mode

**Key outputs**
- builder ID
- region
- project ARN/name
- artifact bucket/prefix
- effective build engine version/capabilities

### 10.1.2 `wharflab_build`

Canonical typed build resource.

**Purpose**
Consumes a builder and build definition/context directly from Terraform/OpenTofu schema.

**Key inputs**
- `builder_id`
- build context source
- definition path or inline definition reference
- tags
- target stage
- platforms
- publication target
- build args
- labels
- secrets
- ssh mounts
- cache import/export config
- push / load semantics
- output mode

**Key outputs**
- image digest
- image reference by digest
- image index digest if multi-platform
- published tags
- metadata about build start/end and backend execution
- normalized context hash

### 10.1.3 `wharflab_bake_build`

Build resource driven by Bake file(s).

**Purpose**
Support existing Bake users while keeping Terraform/OpenTofu in control of remote execution and outputs.

**Key inputs**
- `builder_id`
- one or more Bake files
- selected targets
- variable overrides
- publication overrides allowed by provider contract
- execution settings unsupported by the Bake file itself but relevant to remote backend

**Key outputs**
- per-target digests
- per-target published references
- normalized target expansion metadata

### 10.1.4 `wharflab_compose_build`

Limited interop resource that extracts service build definitions from Compose.

**Purpose**
Allow users with existing Compose files to reuse `services.*.build` for remote image builds.

**Important limitation**
This resource is only for **build extraction**. It must not imply support for full Compose runtime lifecycle semantics.

**Key inputs**
- `builder_id`
- Compose file path(s)
- service selection
- optional `x-bake` handling if present

**Key outputs**
- per-service digests
- build metadata
- normalized build plan

## 10.2 Optional future data sources

- `data.wharflab_bake_plan`
- `data.wharflab_compose_plan`
- `data.wharflab_registry_image`
- `data.wharflab_builder_capabilities`

These help with normalization and inspection without mutating infrastructure.

---

## 11. Build input model

## 11.1 Supported context types

MVP context source types:

1. **Local directory**
2. **Git repository**
3. **Tarball source**
4. **S3 object reference**

Future context source types:

- SSH-authenticated Git
- multiple named contexts
- HTTP remote archive
- registry-based source reference if justified

## 11.2 Recommended Terraform schema

```hcl
resource "wharflab_build" "app" {
  builder_id = wharflab_aws_codebuild_builder.main.id

  context {
    type = "local_directory"
    path = "../app"
  }

  definition {
    dockerfile = "Dockerfile"
  }

  platforms = ["linux/amd64"]
  tags      = ["123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:main"]
}
```

## 11.3 Context normalization requirements

The provider must explicitly define:

- root path resolution rules
- Dockerfile path resolution relative to context root
- ignore file handling (`.dockerignore` and related decisions)
- deterministic archive generation
- file ordering and metadata normalization where feasible
- hashing algorithm used for change detection
- symlink behavior
- large-file handling and size limits

## 11.4 Change detection

The resource should rebuild only when material inputs change. Material inputs include:

- normalized context contents
- build definition contents/path
- build args
- target stage
- platforms
- publication targets/tags if treated as material
- cache policy if it affects outputs
- builder backend selection

The implementation engineer should be careful to distinguish between:

- inputs that change the produced artifact
- inputs that only affect execution environment or observability

---

## 12. Definition model: Dockerfile, Bake, Compose

## 12.1 Dockerfile/Containerfile

This is the canonical low-level build definition.
The provider should support a path-based definition model first.

Inline Dockerfile text is possible but should not be MVP unless there is a compelling use case.

## 12.2 Docker Bake integration

Bake should be the primary third-party declarative integration target.

### Why Bake first

- Build-specific abstraction
- Better fit for multi-target builds
- Natural integration with Buildx ecosystems
- Lower semantic mismatch than Compose

### Expected support model

- Parse Bake file(s)
- Resolve selected targets
- Normalize target metadata into provider-internal build plan
- Execute remotely on provider-managed backend
- Return per-target digests and metadata

### Important design rule

Bake files are **inputs**, not managed stateful resources. The provider should not attempt to “own” the lifecycle of a Bake file.

## 12.3 Compose integration

Compose support should be intentionally limited.

### Supported usage

- Parse build sections from selected services
- Convert build definitions into provider build plan
- Optionally honor `x-bake` extensions where relevant

### Unsupported usage

- full service lifecycle management
- Compose networking semantics
- volume lifecycle
- runtime orchestration

### Reason

Compose is broader than build orchestration. The provider’s job is remote artifact production, not application-topology orchestration.

---

## 13. OCI output model

## 13.1 Output priorities

The provider should treat OCI outputs as first-class results.

### Required outputs

- image digest
- digest-qualified image reference
- published tags

### Optional outputs when applicable

- image index digest
- per-platform child descriptors
- metadata about publication target

## 13.2 Why digest-first

Digest-first outputs are safer and more useful to downstream infrastructure. For example, image-based Lambda deployment should ideally consume immutable references rather than mutable tags.

## 13.3 Future OCI artifact features

The architecture should leave room for:

- SBOM artifacts
- provenance attestations
- signatures
- referrers attached to subject images

This should influence internal metadata modeling even if the MVP does not expose these features yet.

---

## 14. Publication targets and registry model

## 14.1 Registry target concepts

Although the MVP can keep registry handling simple, the design should already separate:

- build execution backend
n- output registry/publication target

A CodeBuild backend may publish to ECR first, but the conceptual model should not permanently assume backend == registry.

## 14.2 MVP publication model

MVP can treat publication target as tags/references supplied directly to the build resource.

Future refinement can introduce a first-class `registry_target` resource if needed.

## 14.3 Registry auth principles

Prefer backend-native auth and cloud-native identity mechanisms.
Avoid coupling the product too tightly to local Docker config parsing.

For AWS MVP, that means leaning on IAM/ECR-native auth in the remote builder environment.

---

## 15. Terraform/OpenTofu UX examples

## 15.1 Typed build example

```hcl
terraform {
  required_providers {
    wharflab = {
      source  = "wharflab/containers"
      version = "~> 0.1"
    }
  }
}

provider "wharflab" {}

resource "wharflab_aws_codebuild_builder" "main" {
  name             = "main"
  project_name     = "wharflab-builder"
  service_role_arn = aws_iam_role.codebuild.arn
  artifact_bucket  = aws_s3_bucket.contexts.bucket
  region           = "eu-west-1"

  environment {
    image        = "aws/codebuild/standard:7.0"
    compute_type = "BUILD_GENERAL1_MEDIUM"
    privileged   = true
  }
}

resource "wharflab_build" "lambda" {
  builder_id = wharflab_aws_codebuild_builder.main.id

  context {
    type = "local_directory"
    path = "../lambda"
  }

  definition {
    dockerfile = "Dockerfile"
  }

  platforms = ["linux/amd64"]
  tags = [
    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-lambda:main"
  ]
}

output "lambda_image_digest" {
  value = wharflab_build.lambda.image_digest
}
```

## 15.2 Bake example

```hcl
resource "wharflab_bake_build" "default" {
  builder_id = wharflab_aws_codebuild_builder.main.id
  files      = ["${path.module}/docker-bake.hcl"]
  targets    = ["image"]
}
```

## 15.3 Compose build extraction example

```hcl
resource "wharflab_compose_build" "services" {
  builder_id = wharflab_aws_codebuild_builder.main.id
  files      = ["${path.module}/compose.yaml"]
  services   = ["api", "worker"]
}
```

---

## 16. Why builds should still be explicit infrastructure artifacts in Terraform/OpenTofu

Although some teams build images outside Terraform/OpenTofu in CI and only deploy digests, this provider intentionally addresses a different use case: users who want **infrastructure-defined remote artifact production**.

This is a better fit when:

- the build backend itself is infrastructure
- the build must run remotely and reproducibly
- the resulting image digest must flow directly into IaC state and downstream resources
- users want a Terraform/OpenTofu-native abstraction for remote builds rather than shelling out

This is materially more infrastructural than simply wrapping `docker build` on a laptop.

---

## 17. Weaknesses in current Docker-provider landscape that justify this provider

This provider exists because the current Docker-provider category has structural limitations for modern remote-build workflows.

### Key observed weak points in the space

1. Historical focus on Docker daemon lifecycle rather than artifact pipelines.
2. Remote operations often inherit transport fragility, especially when tied to daemon or SSH workflows.
3. Registry auth and digest handling can be leaky or awkward in cross-environment setups.
4. Drift semantics become messy across different Docker-compatible runtimes.
5. Build behavior varies too much across local environments.
6. Supply-chain metadata is increasingly expected but often not first-class.
7. Reusable module ergonomics are weaker when auth and daemon assumptions live at provider-global scope.
8. Product scope becomes diluted when runtime object management and modern build pipeline concerns are mixed together.

The WharfLab provider should remain disciplined about its scope and not repeat that design tension.

---

## 18. State model and idempotence

## 18.1 Identity

Builder resources should have stable identities derived from backend resource identity.

Build resources should have identity derived from Terraform/OpenTofu resource instance identity, while their rebuild behavior is driven by material input hashing.

## 18.2 Recommended state fields for build resources

- builder ID
- normalized context hash
- build definition hash
- effective target stage
- effective platforms
- resulting image digest
- resulting image index digest if applicable
- published references
- backend execution identifier
- timestamps and status summary

## 18.3 Refresh behavior

The engineer should define whether refresh:

- merely rehydrates known outputs, or
- actively checks registry/backend for output existence and drift

MVP recommendation: refresh should be conservative and avoid surprising expensive backend calls unless explicitly necessary.

---

## 19. Error handling and user experience

The provider should prefer explicit, actionable errors over backend-native raw failures whenever possible.

### Important error categories

- invalid context path
- missing Dockerfile/Containerfile
- unsupported context type
- unsupported backend capability (for example, multi-platform unavailable)
- failed context upload
- failed registry auth
- failed remote build execution
- digest not discoverable after successful push
- Bake target not found
- Compose service has no build section

### UX principle

When remote execution fails, users should see:

- the backend execution identifier
- the most relevant log location
- the normalized target/context being executed
- which material input hash triggered the rebuild

---

## 20. Security model

## 20.1 Core security principles

- use cloud-native identity where possible
- avoid broad persistent credentials in local config
- make secret and SSH forwarding explicit in schema
- never silently include local machine state in build inputs
- keep build context packaging deterministic and reviewable

## 20.2 AWS-specific security expectations

- CodeBuild service role should be least-privilege
- S3 context bucket should be tightly scoped
- ECR publication permissions should be explicit
- secret injection should use native AWS facilities where possible

---

## 21. Future feature roadmap

## Phase 1 — MVP

- provider skeleton
- Terraform/OpenTofu docs
- `wharflab_aws_codebuild_builder`
- `wharflab_build`
- local directory + S3/Git/tarball contexts
- Dockerfile path support
- digest and tag outputs
- ECR publication
- basic logging metadata
- Terraform Registry and OpenTofu Registry publication

## Phase 2 — ecosystem interop

- `wharflab_bake_build`
- `wharflab_compose_build`
- richer context/data sources
- capability introspection
- improved per-target outputs

## Phase 3 — advanced build features

- cache import/export policies
- multi-platform image index support
- richer registry targets
- more backends

## Phase 4 — supply-chain and artifact expansion

- provenance
- SBOM
- referrers
- signatures
- promotion workflows

---

## 22. Provider naming, namespace, and source address

The organization namespace is **WharfLab**.

Recommended public provider identity:

- Registry namespace: `wharflab`
- Provider type/name: `containers`
- Preferred source address: `wharflab/containers`

### Why this name

- broad enough for future OCI artifact functionality
- specific enough to remain clearly container/build oriented
- avoids overcommitting to Docker or a single cloud

User configuration should therefore look like:

```hcl
terraform {
  required_providers {
    wharflab = {
      source  = "wharflab/containers"
      version = "~> 0.1"
    }
  }
}
```

For registries that require explicit hostnames, the fully qualified source address will include that registry hostname.

---

## 23. Publishing and distribution strategy

## 23.1 Should the provider be consumed directly from GitHub?

No, not as the normal end-user installation model.

The standard Terraform/OpenTofu workflow is for users to declare a provider source address in `required_providers`, and for the CLI to install a compiled provider package from a provider registry.

GitHub should be the **source code and release origin**, but **registries** should be the consumption surface.

## 23.2 Dual-registry publishing strategy

Use one repository and one release pipeline, but publish/register in both ecosystems:

1. **Terraform Registry**
2. **OpenTofu Registry / Search ecosystem**

This is the closest practical analogue to “dual publishing” such as VS Code Marketplace + OpenVSX.

### Recommended strategy

- one GitHub repository: `wharflab/terraform-provider-containers`
- one version stream with SemVer tags
- one GitHub Actions release workflow
- two registry publication/onboarding targets

## 23.3 Recommended repository name

Recommended GitHub repository name:

`wharflab/terraform-provider-containers`

This follows established Terraform provider repository naming conventions.

## 23.4 Expected source addresses

### Terraform

Likely user-facing source address:

```hcl
source = "wharflab/containers"
```

This assumes successful publication in the public Terraform Registry.

### OpenTofu

OpenTofu also uses provider source addresses and supports default public registry addressing. Users may be able to use the same shorthand if the provider is present in the OpenTofu public registry ecosystem.

The implementation team should verify the exact final public source-address expectations during registry onboarding.

## 23.5 Release artifact requirements

For each version, the release pipeline should produce provider binaries for supported platform/architecture combinations, for example:

- linux_amd64
- linux_arm64
- darwin_amd64
- darwin_arm64
- windows_amd64
- windows_arm64 (optional depending on tooling maturity)

The exact packaging format must match the expectations of the Terraform/OpenTofu provider registry protocols and registry onboarding requirements.

## 23.6 Signing and checksums

The release process should include:

- checksums for all provider packages
- a signing key strategy suitable for registry verification
- stable reproducible release asset naming

The implementation/release engineer must align this with the requirements of both Terraform Registry and OpenTofu Registry onboarding.

## 23.7 Documentation publishing

Provider docs should be generated in the format expected by the Terraform/OpenTofu ecosystem.

Recommended approach:

- keep versioned docs in repository
- generate provider/resource/data-source docs during CI
- ensure examples align with real released schema

## 23.8 GitHub Actions release workflow responsibilities

The release workflow should:

1. Trigger on SemVer tags such as `v0.1.0`.
2. Build provider binaries for all supported targets.
3. Generate checksums.
4. Sign release artifacts if required by registry onboarding.
5. Publish a GitHub Release with all assets.
6. Generate/update docs artifacts if appropriate.
7. Perform Terraform Registry publication steps.
8. Perform OpenTofu Registry / Search publication or onboarding steps as required.

## 23.9 Recommended release pipeline shape

### Workflow A — CI on pull requests

- run tests
- run linters
- validate docs generation
- run acceptance tests where feasible

### Workflow B — release on tag

- build binaries
- package artifacts
- sign/checksum
- publish GitHub Release
- update registry-facing metadata/docs if needed

### Workflow C — registry onboarding / maintenance

- one-time or infrequent tasks
- signing key registration
- namespace verification
- registry submission/update workflows

## 23.10 Registry-specific notes for implementation

### Terraform Registry

Implementation engineer should expect:

- GitHub-backed publishing flow
- version discovery via Git tags
- release artifact requirements
- signing key registration / verification requirements
- provider docs rendered from repository content

### OpenTofu Registry

Implementation engineer should expect:

- explicit provider onboarding/submission flow
- signing-key-related submission steps
- provider registry protocol compatibility
- separate registry/search publication lifecycle from Terraform Registry

## 23.11 Operational recommendation

Treat registry publication as a **first-class product concern**, not an afterthought.

This includes:

- durable signing-key ownership
- clear namespace ownership (`wharflab`)
- documented release procedure
- break-glass ownership transfer procedure
- periodic verification that both registries still accept new releases correctly

---

## 24. Suggested implementation package structure

Suggested internal code structure:

```text
internal/
  backend/
    aws_codebuild/
    gcp_cloudbuild/
    azure_acr/
  build/
    context/
    definition/
    normalization/
    planner/
  bake/
  compose/
  oci/
    references/
    digests/
    outputs/
  provider/
  resources/
    aws_codebuild_builder/
    build/
    bake_build/
    compose_build/
  registry/
  release/
```

### Rationale

This structure separates:

- backend integrations
- normalization logic
- OCI output logic
- Terraform/OpenTofu resource implementation
- release/distribution concerns

---

## 25. Suggested MVP implementation order

1. Provider skeleton and docs generation
2. `wharflab_aws_codebuild_builder`
3. local-directory context normalization and hashing
4. Dockerfile-based `wharflab_build`
5. ECR publication and digest capture
6. acceptance tests for AWS backend
7. GitHub Actions release workflow
8. Terraform Registry publication
9. OpenTofu Registry publication
10. Bake support
11. Compose build extraction support

---

## 26. Open engineering questions

1. Should builder resources create/manage cloud backend objects directly, or also support “attach to existing project” modes from day one?
2. How much of the remote worker environment should be provider-managed versus user-supplied image/script?
3. What is the exact deterministic context hashing algorithm and metadata normalization strategy?
4. How much registry introspection should refresh perform by default?
5. Should `wharflab_build` support multiple tags in MVP if only one digest-qualified canonical output is stored?
6. Should Bake parsing be implemented natively, via an embedded tool, or via a normalization sidecar process?
7. Which platform matrix should be officially supported in v0.1.0?
8. How much signing-key / release automation should be fully automated versus manually gated?

---

## 27. Final recommendation summary

Build the WharfLab provider as:

- **one provider**: `wharflab/containers`
- **focused on remote builders and OCI artifact production**
- **CodeBuild-first for MVP**
- **typed Terraform/OpenTofu schema first**
- **Bake as primary external declarative interop**
- **Compose only as limited build-definition extraction**
- **OCI-credible naming on outputs, honest build terminology on inputs**
- **published to both Terraform Registry and OpenTofu Registry from one GitHub Actions release pipeline**

This gives WharfLab a clean, differentiated provider category:

> Build contexts and definitions in, OCI images and artifacts out — using Terraform/OpenTofu-managed remote builders.

---

## 28. Official references for implementation engineer

### Terraform / HashiCorp

- Terraform provider requirements: https://developer.hashicorp.com/terraform/language/providers/requirements
- Terraform providers overview: https://developer.hashicorp.com/terraform/registry/providers
- Terraform provider registry protocol: https://developer.hashicorp.com/terraform/internals/provider-registry-protocol
- Terraform provider publishing: https://developer.hashicorp.com/terraform/registry/providers/publishing
- Terraform provider docs format: https://developer.hashicorp.com/terraform/registry/providers/docs
- Terraform provider release/publish tutorial: https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-release-publish

### OpenTofu

- OpenTofu provider requirements: https://opentofu.org/docs/language/providers/requirements/
- OpenTofu providers overview: https://opentofu.org/docs/language/providers/
- OpenTofu provider registry protocol: https://opentofu.org/docs/internals/provider-registry-protocol/
- OpenTofu registry docs: https://search.opentofu.org/docs/
- OpenTofu provider creation docs: https://search.opentofu.org/docs/providers/creating
- OpenTofu provider adding/onboarding docs: https://search.opentofu.org/docs/providers/adding
- OpenTofu CLI provider installation config: https://opentofu.org/docs/cli/config/config-file/

### Docker / build ecosystem

- Docker build context concepts: https://docs.docker.com/build/concepts/context/
- Dockerfile concepts: https://docs.docker.com/build/concepts/dockerfile/
- Docker Bake overview: https://docs.docker.com/build/bake/
- Docker Bake reference: https://docs.docker.com/build/bake/reference/
- Docker Compose file reference: https://docs.docker.com/reference/compose-file/

### OCI

- OCI image spec: https://specs.opencontainers.org/image-spec/
- OCI descriptor: https://specs.opencontainers.org/image-spec/descriptor/
- OCI manifest: https://specs.opencontainers.org/image-spec/manifest/
