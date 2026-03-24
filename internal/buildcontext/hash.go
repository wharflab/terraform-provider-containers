package buildcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/patternmatcher"
)

// HashDirectory computes a deterministic SHA256 hash of a directory's contents,
// respecting the provided ignore patterns. The hash covers file paths, sizes,
// and contents, ensuring any change to the directory produces a different hash.
func HashDirectory(dir string, pm *patternmatcher.PatternMatcher) (string, error) {
	entries, err := collectHashEntries(dir, pm)
	if err != nil {
		return "", fmt.Errorf("collecting entries for hash: %w", err)
	}

	sort.Strings(entries)

	h := sha256.New()
	for _, relPath := range entries {
		absPath := filepath.Join(dir, relPath)
		info, err := os.Lstat(absPath)
		if err != nil {
			return "", err
		}

		// Hash the relative path and size.
		_, _ = fmt.Fprintf(h, "path:%s\nsize:%d\n", relPath, info.Size())

		if info.Mode().IsRegular() {
			if err := hashFileContents(h, absPath); err != nil {
				return "", err
			}
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func collectHashEntries(dir string, pm *patternmatcher.PatternMatcher) ([]string, error) {
	var entries []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		relPath = filepath.ToSlash(relPath)

		if relPath == ".git" || strings.HasPrefix(relPath, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		ignored, err := ShouldIgnore(pm, relPath, info.IsDir())
		if err != nil {
			return err
		}
		if ignored {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Mode().IsRegular() {
			entries = append(entries, relPath)
		}

		return nil
	})

	return entries, err
}

func hashFileContents(h io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(h, f)
	return err
}
