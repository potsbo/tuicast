package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "history")

	err := appendHistory(path, "vim ./src/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = appendHistory(path, "git switch feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "vim ./src/main.go" {
		t.Errorf("expected first line 'vim ./src/main.go', got %q", lines[0])
	}
	if lines[1] != "git switch feature" {
		t.Errorf("expected second line 'git switch feature', got %q", lines[1])
	}
}
