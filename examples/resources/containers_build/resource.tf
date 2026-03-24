resource "containers_build" "app" {
  builder_id = containers_aws_codebuild_builder.main.id

  context {
    type = "local_directory"
    path = "${path.module}/../../../app"
  }

  definition {
    dockerfile = "Dockerfile"
  }

  platforms = ["linux/amd64"]

  tags = [
    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app:latest",
  ]
}

output "image_digest" {
  value = containers_build.app.image_digest
}

output "image_ref" {
  value = containers_build.app.image_ref
}
