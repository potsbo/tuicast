package main

import (
	"os"
	"testing"
)

func TestFzfCommand_InTmux(t *testing.T) {
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	defer os.Unsetenv("TMUX")

	bin, args := fzfCommand()
	if bin != "fzf-tmux" {
		t.Errorf("expected fzf-tmux, got %q", bin)
	}
	// should have popup args before --
	found := false
	for _, a := range args {
		if a == "--" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected -- separator in fzf-tmux args")
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
