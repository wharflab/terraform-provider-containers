package oci

import (
	"fmt"
	"strings"
)

// Reference represents a parsed OCI image reference.
type Reference struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

// ParseReference parses an OCI image reference string into its components.
// Supports formats:
//   - registry/repo:tag
//   - registry/repo@sha256:...
//   - registry/repo:tag@sha256:...
//   - repo:tag (defaults to docker.io)
func ParseReference(ref string) (*Reference, error) {
	if ref == "" {
		return nil, fmt.Errorf("empty reference")
	}

	r := &Reference{}

	// Split off digest.
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		r.Digest = ref[idx+1:]
		ref = ref[:idx]
	}

	// Split off tag.
	// Find the last colon that is not part of a port number.
	// A tag separator comes after the last slash.
	lastSlash := strings.LastIndex(ref, "/")
	colonIdx := strings.LastIndex(ref, ":")
	if colonIdx > lastSlash {
		r.Tag = ref[colonIdx+1:]
		ref = ref[:colonIdx]
	}

	// Now ref is registry/repository or just repository.
	// Determine if the first component is a registry (contains a dot or colon, or is "localhost").
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// No slash: assume docker.io library image.
		r.Registry = "docker.io"
		r.Repository = "library/" + parts[0]
	} else if isRegistryHost(parts[0]) {
		r.Registry = parts[0]
		r.Repository = parts[1]
	} else {
		// No registry detected: assume docker.io.
		r.Registry = "docker.io"
		r.Repository = ref
	}

	if r.Tag == "" && r.Digest == "" {
		r.Tag = "latest"
	}

	return r, nil
}

func isRegistryHost(s string) bool {
	return strings.Contains(s, ".") || strings.Contains(s, ":") || s == "localhost"
}

// String formats the reference back to a string.
func (r *Reference) String() string {
	s := r.Registry + "/" + r.Repository
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	if r.Digest != "" {
		s += "@" + r.Digest
	}
	return s
}

// WithDigest returns a new reference string with the given digest appended.
func (r *Reference) WithDigest(digest string) string {
	s := r.Registry + "/" + r.Repository
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	return s + "@" + digest
}

// IsECR returns true if the reference points to an ECR registry.
func (r *Reference) IsECR() bool {
	return strings.Contains(r.Registry, ".dkr.ecr.") && strings.Contains(r.Registry, ".amazonaws.com")
}

// ECRRegion extracts the AWS region from an ECR registry host.
// Returns an error if the registry is not an ECR host.
func (r *Reference) ECRRegion() (string, error) {
	return ExtractECRRegion(r.Registry)
}

// ExtractECRRegion extracts the AWS region from an ECR registry hostname.
// e.g. "123456789012.dkr.ecr.eu-west-1.amazonaws.com" -> "eu-west-1"
func ExtractECRRegion(registry string) (string, error) {
	// Format: {account}.dkr.ecr.{region}.amazonaws.com
	parts := strings.Split(registry, ".")
	if len(parts) < 6 || parts[1] != "dkr" || parts[2] != "ecr" {
		return "", fmt.Errorf("not an ECR registry: %s", registry)
	}
	// Region is everything between "ecr." and ".amazonaws.com"
	// which is parts[3] (could be multi-segment like cn-north-1)
	region := parts[3]
	return region, nil
}
