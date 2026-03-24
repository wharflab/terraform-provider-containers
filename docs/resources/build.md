---
page_title: "containers_build Resource - WharfLab Containers"
subcategory: ""
description: |-
  Executes a remote container build and publishes OCI images.
---

# containers_build (Resource)

Executes a remote container build using a configured builder backend and publishes OCI images to a container registry. The resource automatically detects context changes via content hashing and triggers rebuilds when inputs change.

## Example Usage

### Basic Dockerfile build

```hcl
resource "containers_build" "app" {
  builder_id = containers_aws_codebuild_builder.main.id

  context {
    type = "local_directory"
    path = "${path.module}/../app"
  }

  definition {
    dockerfile = "Dockerfile"
  }

  platforms = ["linux/amd64"]

  tags = [
    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app:latest",
  ]
}
```

### Multi-tag build with build args

```hcl
resource "containers_build" "api" {
  builder_id = containers_aws_codebuild_builder.main.id

  context {
    type = "local_directory"
    path = "${path.module}/../api"
  }

  definition {
    dockerfile = "Dockerfile.production"
    target     = "runtime"
  }

  build_args = {
    GO_VERSION = "1.23"
    APP_ENV    = "production"
  }

  labels = {
    "org.opencontainers.image.source" = "https://github.com/myorg/api"
  }

  tags = [
    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/api:${var.git_sha}",
    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/api:latest",
  ]
}
```

### Using the digest output with downstream resources

```hcl
resource "aws_lambda_function" "handler" {
  function_name = "my-handler"
  package_type  = "Image"
  image_uri     = containers_build.app.image_ref
  role          = aws_iam_role.lambda.arn
}
```

## Argument Reference

- `builder_id` (Required) - ID of the builder resource to execute the build.
- `tags` (Required) - List of image tags/references to publish.
- `platforms` (Optional) - Target platforms. Defaults to `["linux/amd64"]`.
- `build_args` (Optional) - Map of Docker build arguments.
- `labels` (Optional) - Map of image labels.
- `push` (Optional) - Push built image to registry. Defaults to `true`.

### context

Required block. Defines the build context source.

- `type` (Required) - Context source type: `local_directory` or `s3`.
- `path` (Optional) - Local directory path. Required when `type = "local_directory"`.
- `bucket` (Optional) - S3 bucket. Required when `type = "s3"`.
- `key` (Optional) - S3 object key. Required when `type = "s3"`.

### definition

Optional block. Defaults to `Dockerfile` in the context root.

- `dockerfile` (Optional) - Path to the Dockerfile relative to the context root. Defaults to `Dockerfile`.
- `target` (Optional) - Target build stage name.

## Attribute Reference

- `id` - Unique resource identifier.
- `image_digest` - Image digest (e.g. `sha256:abc123...`).
- `image_ref` - Digest-qualified image reference (e.g. `registry/repo:tag@sha256:abc123...`).
- `image_index_digest` - Image index digest (for future multi-platform builds).
- `context_hash` - SHA256 hash of the build context contents. Changes to this value trigger rebuilds.
- `build_id` - Backend build execution ID (CodeBuild build ID).
- `build_status` - Final build status.
- `build_start_time` - Build start time (RFC 3339).
- `build_end_time` - Build end time (RFC 3339).
- `log_url` - URL to the build log in CloudWatch.

## Change Detection

The provider computes a deterministic SHA256 hash of the build context during planning. This hash covers:

- All file paths (sorted alphabetically)
- File sizes
- File contents

The `.git` directory and files matching `.dockerignore` patterns are excluded from the hash.

When the hash changes between plan and state, Terraform plans an update which triggers a rebuild.
