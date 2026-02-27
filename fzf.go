package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

var ErrCancelled = errors.New("cancelled")

type FzfOptions struct {
	Preview   string
	Delimiter string
	WithNth   string
	Header    string
	Prompt    string
	Bind      []string
	PrintQuery bool
}

type FzfResult struct {
	Index int
	Line  string
	Query string
}

func fzfSelect(items []string, opts FzfOptions) (*FzfResult, error) {
	args := []string{"--ansi"}

	if opts.Preview != "" {
		args = append(args, "--preview", opts.Preview)
	}
	if opts.Delimiter != "" {
		args = append(args, "--delimiter", opts.Delimiter)
	}
	if opts.WithNth != "" {
		args = append(args, "--with-nth", opts.WithNth)
	}
	if opts.Header != "" {
		args = append(args, "--header", opts.Header)
	}
	if opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}
	if opts.PrintQuery {
		args = append(args, "--print-query")
	}
	for _, b := range opts.Bind {
		args = append(args, "--bind", b)
	}

	cmd := exec.Command("fzf", args...)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CLICOLOR_FORCE=1")

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 1 {
				return nil, ErrCancelled
			}
		}
		return nil, err
	}

	output := strings.TrimRight(string(out), "\n")

	if opts.PrintQuery {
		parts := strings.SplitN(output, "\n", 2)
		query := parts[0]
		return &FzfResult{Query: query}, nil
	}

	selected := output
	for i, item := range items {
		if item == selected {
			return &FzfResult{Index: i, Line: selected}, nil
		}
	}
	return &FzfResult{Index: -1, Line: selected}, nil
}

func fzfTextInput(placeholder string) (string, error) {
	result, err := fzfSelect(nil, FzfOptions{
		PrintQuery: true,
		Prompt:     placeholder + ": ",
	})
	if err != nil {
		return "", err
	}
	return result.Query, nil
}
