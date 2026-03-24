package buildcontext

import (
	"fmt"
	"os"
)

// PrepareLocalContext prepares a local directory as a build context. It reads
// .dockerignore rules, computes the content hash, and creates a deterministic
// tar.gz archive ready for upload.
func PrepareLocalContext(dir string) (*PreparedContext, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("context directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("context path %q is not a directory", dir)
	}

	patterns, err := ReadDockerignore(dir)
	if err != nil {
		return nil, fmt.Errorf("reading .dockerignore: %w", err)
	}

	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		return nil, fmt.Errorf("parsing .dockerignore patterns: %w", err)
	}

	hash, err := HashDirectory(dir, pm)
	if err != nil {
		return nil, fmt.Errorf("hashing context: %w", err)
	}

	archivePath, err := CreateArchive(dir, pm)
	if err != nil {
		return nil, fmt.Errorf("creating context archive: %w", err)
	}

	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("stat archive: %w", err)
	}

	return &PreparedContext{
		Hash:        hash,
		ArchivePath: archivePath,
		Size:        archiveInfo.Size(),
	}, nil
}

// HashLocalContext computes the content hash for a local directory without
// creating an archive. Used during plan to detect changes.
func HashLocalContext(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("context directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("context path %q is not a directory", dir)
	}

	patterns, err := ReadDockerignore(dir)
	if err != nil {
		return "", fmt.Errorf("reading .dockerignore: %w", err)
	}

	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		return "", fmt.Errorf("parsing .dockerignore patterns: %w", err)
	}

	return HashDirectory(dir, pm)
}
