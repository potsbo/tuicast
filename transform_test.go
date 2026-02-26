package main

import (
	"testing"
)

func TestTransform_PerItem(t *testing.T) {
	lines := []string{"/path/to/foo.txt", "/path/to/bar.txt"}
	result, err := transform("basename {}", lines, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0] != "foo.txt" {
		t.Errorf("expected 'foo.txt', got %q", result[0])
	}
	if result[1] != "bar.txt" {
		t.Errorf("expected 'bar.txt', got %q", result[1])
	}
}

func TestTransform_Pipe(t *testing.T) {
	lines := []string{"/path/to/foo.txt", "/path/to/bar.txt"}
	result, err := transform("| sed 's|.*/||'", lines, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	if result[0] != "foo.txt" {
		t.Errorf("expected 'foo.txt', got %q", result[0])
	}
	if result[1] != "bar.txt" {
		t.Errorf("expected 'bar.txt', got %q", result[1])
	}
}

func TestTransform_PipeLineCountMismatch(t *testing.T) {
	lines := []string{"a", "b", "c"}
	_, err := transform("| head -1", lines, nil)
	if err == nil {
		t.Fatal("expected error for line count mismatch")
	}
}
