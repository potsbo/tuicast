package main

import (
	"testing"
)

func TestShellRun(t *testing.T) {
	out, err := shellOutput("echo hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got %q", out)
	}
}

func TestShellRunWithEnv(t *testing.T) {
	out, err := shellOutput("echo $FOO", []string{"FOO=bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "bar" {
		t.Errorf("expected 'bar', got %q", out)
	}
}

func TestShellLines(t *testing.T) {
	lines, err := shellLines("printf 'a\\nb\\nc'", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("unexpected lines: %v", lines)
	}
}
