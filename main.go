package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var defaultConfigYAML = `# tuicast example config
# Demonstrates all view types and features.

views:
  # MenuView: lists views by title, navigate with Enter
  main:
    menu: [all-files, files, grep, colors, hello]

  # UnionView: merges items from multiple 1-step FormViews
  all-files:
    title: All files (union)
    union: [files, scripts]

  # FormView 1-step + per-item display + preview
  files:
    title: Files
    steps:
      - name: file
        sources:
          - list: "find . -maxdepth 3 -type f -not -path './.git/*'"
            display: "basename {}"
            preview: "head -20 {}"
    run: "echo Selected: $file"

  # FormView 1-step + pipe display
  scripts:
    title: Shell scripts
    steps:
      - name: script
        sources:
          - list: "find . -name '*.sh' -not -path './.git/*' 2>/dev/null; echo hello.sh"
            display: "| sed 's|.*/||'"
    run: "echo Selected: $script"

  # FormView n-step (wizard)
  grep:
    title: Grep (wizard)
    steps:
      - name: pattern
        sources:
          - input: Search pattern
      - name: file
        sources:
          - list: "grep -rl \"$pattern\" . 2>/dev/null || true"
            preview: "grep -n --color=always \"$pattern\" {}"
    run: "echo Found $pattern in $file"

  # FormView 1-step with pipe display
  colors:
    title: Colors
    steps:
      - name: color
        sources:
          - list: "printf 'red\ngreen\nblue\nyellow\ncyan'"
            display: "| awk '{print $0}'"
    run: "echo You picked $color"

  # FormView 0-step (leaf)
  hello:
    title: Hello World
    run: "echo Hello from tuicast!"
`

func main() {
	home, _ := os.UserHomeDir()
	defaultConfig := filepath.Join(home, ".config", "tuicast", "config.yaml")

	configPath := flag.String("c", defaultConfig, "config file path")
	viewName := flag.String("view", "main", "view to open")
	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		if isDefaultConfigMissing(*configPath, err) {
			if handleMissingConfig(*configPath) {
				data, err = os.ReadFile(*configPath)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = executeView(cfg, *viewName)
	if err != nil {
		if err == ErrCancelled {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func isDefaultConfigMissing(path string, err error) bool {
	if path == "" {
		return false
	}
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, ".config", "tuicast", "config.yaml")
	return path == defaultPath && errors.Is(err, os.ErrNotExist)
}

func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfigYAML), 0o644)
}

func handleMissingConfig(path string) bool {
	items := []string{
		"init\tCreate default config",
		"exit\tExit",
	}

	opts := FzfOptions{
		Delimiter: "\t",
		WithNth:   "2",
		Header:    "Config not found: " + path,
		Preview:   fmt.Sprintf("echo '%s'", defaultConfigYAML),
	}

	result, err := fzfSelect(items, opts)
	if err != nil {
		return false
	}

	selected := items[result.Index]
	if len(selected) >= 4 && selected[:4] == "init" {
		if err := writeDefaultConfig(path); err != nil {
			fmt.Fprintf(os.Stderr, "error creating config: %v\n", err)
			return false
		}
		fmt.Fprintf(os.Stderr, "Created %s\n", path)
		return true
	}
	return false
}
