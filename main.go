package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	home, _ := os.UserHomeDir()
	defaultConfig := filepath.Join(home, ".config", "tuicast", "config.yaml")

	configPath := flag.String("c", defaultConfig, "config file path")
	viewName := flag.String("view", "main", "view to open")
	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := ParseConfig(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := executeView(cfg, *viewName); err != nil {
		if err == ErrCancelled {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
