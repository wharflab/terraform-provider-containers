package buildcontext

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moby/patternmatcher"
)

// Epoch is the fixed timestamp used for deterministic archives.
var Epoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// CreateArchive creates a deterministic tar.gz archive of the given directory,
// respecting the provided ignore patterns. It writes to a temporary file and
// returns its path. The caller is responsible for removing the temp file.
func CreateArchive(dir string, pm *patternmatcher.PatternMatcher) (string, error) {
	tmpFile, err := os.CreateTemp("", "wharflab-context-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if err := writeArchive(tmpFile, dir, pm); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func writeArchive(w io.Writer, dir string, pm *patternmatcher.PatternMatcher) error {
	gzw := gzip.NewWriter(w)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	entries, err := collectEntries(dir, pm)
	if err != nil {
		return err
	}

	sort.Strings(entries)

	for _, relPath := range entries {
		absPath := filepath.Join(dir, relPath)
		if err := addToArchive(tw, absPath, relPath); err != nil {
			return err
		}
	}

	return nil
}

func collectEntries(dir string, pm *patternmatcher.PatternMatcher) ([]string, error) {
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

		// Normalize to forward slashes for pattern matching.
		relPath = filepath.ToSlash(relPath)

		// Always skip .git directory.
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

		if info.Mode().IsRegular() || info.IsDir() {
			entries = append(entries, relPath)
		}

		return nil
	})

	return entries, err
}

func addToArchive(tw *tar.Writer, absPath, relPath string) error {
	info, err := os.Lstat(absPath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	// Deterministic: use forward slashes and fixed timestamp.
	header.Name = filepath.ToSlash(relPath)
	header.ModTime = Epoch
	header.AccessTime = Epoch
	header.ChangeTime = Epoch
	// Normalize ownership.
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}
