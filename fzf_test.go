package main

import (
	"os"
	"testing"
)

func TestFzfCommand_InTmux(t *testing.T) {
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	defer os.Unsetenv("TMUX")

	bin, args := fzfCommand()
	if bin != "fzf" {
		t.Errorf("expected fzf, got %q", bin)
	}
	found := false
	for _, a := range args {
		if a == "--tmux" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --tmux flag in args")
	}
}

func TestFzfCommand_OutsideTmux(t *testing.T) {
	os.Unsetenv("TMUX")

	bin, args := fzfCommand()
	if bin != "fzf" {
		t.Errorf("expected fzf, got %q", bin)
	}
	if len(args) != 0 {
		t.Errorf("expected no extra args, got %v", args)
	}
}
