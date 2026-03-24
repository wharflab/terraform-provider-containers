package buildcontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDockerignore_Missing(t *testing.T) {
	dir := t.TempDir()
	patterns, err := ReadDockerignore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if patterns != nil {
		t.Errorf("expected nil, got %v", patterns)
	}
}

func TestReadDockerignore_Parses(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nnode_modules\n\n*.log\n!important.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns, err := ReadDockerignore(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"node_modules", "*.log", "!important.log"}
	if len(patterns) != len(expected) {
		t.Fatalf("got %d patterns, want %d", len(patterns), len(expected))
	}
	for i, p := range patterns {
		if p != expected[i] {
			t.Errorf("pattern %d: got %q, want %q", i, p, expected[i])
		}
	}
}

func TestShouldIgnore(t *testing.T) {
	pm, err := NewPatternMatcher([]string{"*.log", "node_modules"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"app.log", false, true},
		{"main.go", false, false},
		{"node_modules", true, true},
		{"src/main.go", false, false},
	}

	for _, tt := range tests {
		got, err := ShouldIgnore(pm, tt.path, tt.isDir)
		if err != nil {
			t.Errorf("ShouldIgnore(%q): %v", tt.path, err)
			continue
		}
		if got != tt.expect {
			t.Errorf("ShouldIgnore(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.expect)
		}
	}
}

func TestShouldIgnore_NilMatcher(t *testing.T) {
	got, err := ShouldIgnore(nil, "anything", false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("nil matcher should never ignore")
	}
}
