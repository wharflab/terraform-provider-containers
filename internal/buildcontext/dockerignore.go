package buildcontext

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/patternmatcher"
)

// ReadDockerignore reads a .dockerignore file from the given directory and
// returns the parsed patterns. Returns an empty slice if the file does not exist.
func ReadDockerignore(dir string) ([]string, error) {
	p := filepath.Join(dir, ".dockerignore")

	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

// NewPatternMatcher creates a patternmatcher from dockerignore patterns.
func NewPatternMatcher(patterns []string) (*patternmatcher.PatternMatcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	return patternmatcher.New(patterns)
}

// ShouldIgnore checks whether a given relative path should be ignored based on
// the provided PatternMatcher. If pm is nil, nothing is ignored.
func ShouldIgnore(pm *patternmatcher.PatternMatcher, relPath string, isDir bool) (bool, error) {
	if pm == nil {
		return false, nil
	}
	if isDir {
		relPath += "/"
	}
	return pm.MatchesOrParentMatches(relPath)
}
