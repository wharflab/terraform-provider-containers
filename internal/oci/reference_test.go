package oci

import (
	"testing"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantReg  string
		wantRepo string
		wantTag  string
		wantDig  string
		wantErr  bool
	}{
		{
			name:     "ECR with tag",
			input:    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app:latest",
			wantReg:  "123456789012.dkr.ecr.eu-west-1.amazonaws.com",
			wantRepo: "my-app",
			wantTag:  "latest",
		},
		{
			name:     "ECR with digest",
			input:    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app@sha256:abcdef1234567890",
			wantReg:  "123456789012.dkr.ecr.eu-west-1.amazonaws.com",
			wantRepo: "my-app",
			wantDig:  "sha256:abcdef1234567890",
		},
		{
			name:     "ECR with tag and digest",
			input:    "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app:v1@sha256:abcdef",
			wantReg:  "123456789012.dkr.ecr.eu-west-1.amazonaws.com",
			wantRepo: "my-app",
			wantTag:  "v1",
			wantDig:  "sha256:abcdef",
		},
		{
			name:     "docker hub short",
			input:    "nginx",
			wantReg:  "docker.io",
			wantRepo: "library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "docker hub with user",
			input:    "myuser/myimage:v2",
			wantReg:  "docker.io",
			wantRepo: "myuser/myimage",
			wantTag:  "v2",
		},
		{
			name:     "ghcr.io",
			input:    "ghcr.io/owner/repo:sha-abc123",
			wantReg:  "ghcr.io",
			wantRepo: "owner/repo",
			wantTag:  "sha-abc123",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:     "nested repository",
			input:    "123456789012.dkr.ecr.us-east-1.amazonaws.com/team/service/app:main",
			wantReg:  "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			wantRepo: "team/service/app",
			wantTag:  "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.Registry != tt.wantReg {
				t.Errorf("registry: got %q, want %q", ref.Registry, tt.wantReg)
			}
			if ref.Repository != tt.wantRepo {
				t.Errorf("repository: got %q, want %q", ref.Repository, tt.wantRepo)
			}
			if ref.Tag != tt.wantTag {
				t.Errorf("tag: got %q, want %q", ref.Tag, tt.wantTag)
			}
			if ref.Digest != tt.wantDig {
				t.Errorf("digest: got %q, want %q", ref.Digest, tt.wantDig)
			}
		})
	}
}

func TestReference_IsECR(t *testing.T) {
	ref, _ := ParseReference("123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:latest")
	if !ref.IsECR() {
		t.Error("expected ECR reference")
	}

	ref2, _ := ParseReference("ghcr.io/owner/repo:latest")
	if ref2.IsECR() {
		t.Error("expected non-ECR reference")
	}
}

func TestExtractECRRegion(t *testing.T) {
	region, err := ExtractECRRegion("123456789012.dkr.ecr.eu-west-1.amazonaws.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region != "eu-west-1" {
		t.Errorf("got %q, want %q", region, "eu-west-1")
	}

	_, err = ExtractECRRegion("ghcr.io")
	if err == nil {
		t.Error("expected error for non-ECR registry")
	}
}

func TestReference_WithDigest(t *testing.T) {
	ref, _ := ParseReference("123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:v1")
	got := ref.WithDigest("sha256:abc123")
	want := "123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:v1@sha256:abc123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
