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
