package main

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
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

func TestRunScriptTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	script := runScript("echo hi")
	if !strings.Contains(script, "tmux display-popup") {
		t.Errorf("expected tmux wrapper, got: %s", script)
	}
}

// Outside tmux the wrapper must pass exit codes through, and must not block
// waiting for a keypress when no tty is available (Setsid detaches the test's
// controlling tty, like a headless run).
func TestRunScriptExitCodesWithoutTTY(t *testing.T) {
	t.Setenv("TMUX", "")
	cases := []struct {
		cmd  string
		want int
	}{
		{"true", 0},
		{"exit 7", 7},
		{"exit 130", 130},
	}
	for _, tc := range cases {
		c := exec.Command("sh", "-c", runScript(tc.cmd))
		c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err := c.Run()
		got := 0
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("%q: unexpected error: %v", tc.cmd, err)
			}
			got = exitErr.ExitCode()
		}
		if got != tc.want {
			t.Errorf("%q: expected exit %d, got %d", tc.cmd, tc.want, got)
		}
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
