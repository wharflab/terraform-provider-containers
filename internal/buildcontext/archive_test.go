package buildcontext

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateArchive_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa")
	writeFile(t, dir, "b.txt", "bbb")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	writeFile(t, dir, "sub/c.txt", "ccc")

	path1, err := CreateArchive(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path1)

	path2, err := CreateArchive(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path2)

	entries1 := listArchiveEntries(t, path1)
	entries2 := listArchiveEntries(t, path2)

	if len(entries1) != len(entries2) {
		t.Fatalf("entry count mismatch: %d vs %d", len(entries1), len(entries2))
	}
	for i := range entries1 {
		if entries1[i] != entries2[i] {
			t.Errorf("entry %d: %q vs %q", i, entries1[i], entries2[i])
		}
	}
}

func TestCreateArchive_SortedEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "z.txt", "z")
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "m.txt", "m")

	path, err := CreateArchive(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	entries := listArchiveEntries(t, path)
	for i := 1; i < len(entries); i++ {
		if entries[i] < entries[i-1] {
			t.Errorf("entries not sorted: %v", entries)
			break
		}
	}
}

func TestCreateArchive_ExcludesGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "content")
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")

	path, err := CreateArchive(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	for _, entry := range listArchiveEntries(t, path) {
		if entry == ".git" || entry == ".git/HEAD" || entry == ".git/objects" {
			t.Errorf("archive should not contain .git entries, found: %s", entry)
		}
	}
}

func TestCreateArchive_RespectsIgnorePatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.go", "package main")
	writeFile(t, dir, "ignore.log", "log")

	pm, err := NewPatternMatcher([]string{"*.log"})
	if err != nil {
		t.Fatal(err)
	}

	path, err := CreateArchive(dir, pm)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	entries := listArchiveEntries(t, path)
	for _, e := range entries {
		if e == "ignore.log" {
			t.Error("archive should not contain ignored file")
		}
	}
}

func TestCreateArchive_NormalizedTimestamps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "content")

	path, err := CreateArchive(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !hdr.ModTime.Equal(Epoch) {
			t.Errorf("entry %q has non-epoch mod time: %v", hdr.Name, hdr.ModTime)
		}
	}
}

func listArchiveEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	var entries []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, hdr.Name)
	}
	return entries
}
