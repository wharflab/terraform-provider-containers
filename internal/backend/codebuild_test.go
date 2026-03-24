package backend

import (
	"strings"
	"testing"
)

func TestGenerateBuildspec_ContainsRequiredSections(t *testing.T) {
	req := &BuildRequest{
		ProjectName:   "test-project",
		ContextBucket: "my-bucket",
		ContextKey:    "contexts/abc123.tar.gz",
		Dockerfile:    "Dockerfile",
		Platforms:     []string{"linux/amd64"},
		Tags:          []string{"123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:latest"},
		ResultBucket:  "my-bucket",
		ResultKey:     "results/test.json",
	}

	spec := GenerateBuildspec(req)

	if !strings.Contains(spec, "version: 0.2") {
		t.Error("buildspec should contain version header")
	}
	if !strings.Contains(spec, "pre_build") {
		t.Error("buildspec should contain pre_build phase")
	}
	if !strings.Contains(spec, "docker buildx build") {
		t.Error("buildspec should contain docker buildx build command")
	}
	if !strings.Contains(spec, "--push") {
		t.Error("buildspec should contain --push flag")
	}
	if !strings.Contains(spec, "--metadata-file") {
		t.Error("buildspec should contain --metadata-file flag")
	}
	if !strings.Contains(spec, "WHARFLAB_RESULT_BUCKET") {
		t.Error("buildspec should reference result bucket env var")
	}
	if !strings.Contains(spec, "aws s3 cp") {
		t.Error("buildspec should upload result metadata to S3")
	}
}

func TestGenerateBuildspec_ECRLogin(t *testing.T) {
	req := &BuildRequest{
		Dockerfile: "Dockerfile",
		Platforms:  []string{"linux/amd64"},
		Tags:       []string{"123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:latest"},
	}

	spec := GenerateBuildspec(req)

	if !strings.Contains(spec, "ecr get-login-password") {
		t.Error("buildspec should contain ECR login command")
	}
	if !strings.Contains(spec, "docker login") {
		t.Error("buildspec should contain docker login command")
	}
}
