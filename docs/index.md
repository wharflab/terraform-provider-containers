---
page_title: "WharfLab Containers Provider"
subcategory: ""
description: |-
  The WharfLab Containers provider enables Terraform-managed remote container builds and OCI artifact production.
---

# WharfLab Containers Provider

The WharfLab Containers provider is a Terraform/OpenTofu provider for **remote container builds** and **OCI artifact production** using cloud-native build backends.

Unlike Docker-daemon-oriented providers, this provider focuses on:

- **Remote isolated builds** via cloud builder backends
- **Immutable OCI outputs** with digest-first references
- **Deterministic build context** packaging and change detection
- **Cloud-native auth** using IAM and service roles

## AWS CodeBuild Backend (MVP)

The initial release supports AWS CodeBuild as the remote build backend. CodeBuild provides:

- Managed, isolated build environments
- IAM-based authentication
- ECR integration for image publication
- S3-based context upload
- CloudWatch logging

## Example Usage

```hcl
terraform {
  required_providers {
    containers = {
      source  = "wharflab/containers"
      version = "~> 0.1"
    }
  }
}

provider "containers" {
  region = "eu-west-1"
}

resource "containers_aws_codebuild_builder" "main" {
  name             = "main"
  project_name     = "my-builder"
  service_role_arn = aws_iam_role.codebuild.arn
  artifact_bucket  = aws_s3_bucket.contexts.bucket

  environment {
    image        = "aws/codebuild/standard:7.0"
    compute_type = "BUILD_GENERAL1_MEDIUM"
    privileged   = true
  }
}

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
  tags      = ["123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:latest"]
}

output "image_digest" {
  value = containers_build.app.image_digest
}
```

## Authentication

The provider uses the standard AWS credential chain. You can configure credentials via:

- Provider attributes (`access_key`, `secret_key`, `region`, `profile`)
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`)
- Shared credentials file (`~/.aws/credentials`)
- IAM instance profile / ECS task role / IRSA

{{ .SchemaMarkdown | trimspace }}
