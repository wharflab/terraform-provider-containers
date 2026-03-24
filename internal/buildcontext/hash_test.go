package buildcontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashDirectory_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "go.mod", "module test\n")

	hash1, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	hash2, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("hashes differ: %s vs %s", hash1, hash2)
	}
}

func TestHashDirectory_ChangesOnContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello")

	hash1, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "file.txt", "world")

	hash2, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("hash should change when file content changes")
	}
}

func TestHashDirectory_ChangesOnNewFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "hello")

	hash1, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "b.txt", "world")

	hash2, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("hash should change when a file is added")
	}
}

func TestHashDirectory_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello")
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")

	hash1, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Modify .git content — hash should not change.
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/other\n")

	hash2, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Error("hash should not change when .git content changes")
	}
}

func TestHashDirectory_RespectsPatternMatcher(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.txt", "keep")
	writeFile(t, dir, "ignore.log", "log data")

	pm, err := NewPatternMatcher([]string{"*.log"})
	if err != nil {
		t.Fatal(err)
	}

	hashWithIgnore, err := HashDirectory(dir, pm)
	if err != nil {
		t.Fatal(err)
	}

	hashWithout, err := HashDirectory(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if hashWithIgnore == hashWithout {
		t.Error("hash should differ when patterns exclude files")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
