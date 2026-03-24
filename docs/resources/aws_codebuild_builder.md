---
page_title: "containers_aws_codebuild_builder Resource - WharfLab Containers"
subcategory: ""
description: |-
  Manages an AWS CodeBuild project as a remote container build backend.
---

# containers_aws_codebuild_builder (Resource)

Manages an AWS CodeBuild project configured as a remote container build backend. This resource creates and manages the CodeBuild project that `containers_build` resources use to execute remote builds.

The CodeBuild project is configured with:
- An S3 source type for receiving build context archives
- No artifact output (builds push directly to container registries)
- Privileged mode for Docker-in-Docker builds

## Example Usage

```hcl
resource "containers_aws_codebuild_builder" "main" {
  name             = "main"
  project_name     = "wharflab-container-builder"
  service_role_arn = aws_iam_role.codebuild.arn
  artifact_bucket  = aws_s3_bucket.build_contexts.bucket
  region           = "eu-west-1"

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

## Argument Reference

- `name` (Required) - Logical name for this builder.
- `project_name` (Optional) - CodeBuild project name. Defaults to `wharflab-{name}`. Changing this forces a new resource.
- `service_role_arn` (Required) - IAM service role ARN for the CodeBuild project.
- `artifact_bucket` (Required) - S3 bucket for build context uploads and build metadata.
- `artifact_prefix` (Optional) - S3 key prefix. Defaults to `wharflab/`.
- `region` (Optional) - AWS region override. Defaults to the provider region.
- `tags` (Optional) - Map of tags to apply to the CodeBuild project.

### environment

Required block.

- `image` (Required) - Docker image for the build environment (e.g. `aws/codebuild/standard:7.0`).
- `compute_type` (Required) - Compute type (`BUILD_GENERAL1_SMALL`, `BUILD_GENERAL1_MEDIUM`, `BUILD_GENERAL1_LARGE`).
- `privileged` (Optional) - Enable privileged mode. Defaults to `true`.

### logging

Optional block.

- `cloudwatch_enabled` (Optional) - Enable CloudWatch logging. Defaults to `true`.
- `log_group_name` (Optional) - CloudWatch log group name.
- `stream_name` (Optional) - CloudWatch log stream name prefix.

### vpc_config

Optional block.

- `vpc_id` (Required) - VPC ID.
- `subnets` (Required) - List of subnet IDs.
- `security_group_ids` (Required) - List of security group IDs.

## Attribute Reference

- `id` - Builder identifier (the CodeBuild project name).
- `project_arn` - ARN of the CodeBuild project.

## Import

Import using the CodeBuild project name:

```shell
terraform import containers_aws_codebuild_builder.main wharflab-container-builder
```
