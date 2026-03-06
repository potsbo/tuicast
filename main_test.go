package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_IsValidYAML(t *testing.T) {
	cfg, err := ParseConfig([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatalf("default config is not valid: %v", err)
	}

	// main should be a MenuView (entry point)
	main := cfg.Views["main"]
	if len(main.Menu) == 0 {
		t.Error("main should be a MenuView")
	}

	hasFormView0Step := false  // leaf (0 step)
	hasFormView1Step := false  // 1 step
	hasFormViewNStep := false  // n step (wizard)
	hasUnionView := false
	hasInputStep := false
	hasPerItemDisplay := false
	hasPipeDisplay := false
	hasPreview := false

	for _, v := range cfg.Views {
		if v.Run != "" {
			switch len(v.Form) {
			case 0:
				hasFormView0Step = true
			case 1:
				hasFormView1Step = true
			default:
				hasFormViewNStep = true
			}
			for _, s := range v.Form {
				for _, src := range s.Sources {
					if src.Input != "" {
						hasInputStep = true
					}
					if src.Display != "" {
						if src.Display[0] == '|' {
							hasPipeDisplay = true
						} else {
							hasPerItemDisplay = true
						}
					}
					if src.Preview != "" {
						hasPreview = true
					}
				}
			}
		}
		if len(v.Union) > 0 {
			hasUnionView = true
		}
	}

	checks := map[string]bool{
		"FormView 0-step (leaf)":    hasFormView0Step,
		"FormView 1-step":           hasFormView1Step,
		"FormView n-step (wizard)":  hasFormViewNStep,
		"UnionView":                 hasUnionView,
		"InputStep":                 hasInputStep,
		"per-item display ({})":     hasPerItemDisplay,
		"pipe display (|)":          hasPipeDisplay,
		"preview":                   hasPreview,
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("default config should demonstrate %s", name)
		}
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	err := writeDefaultConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error reading file: %v", err)
	}

	_, err = ParseConfig(data)
	if err != nil {
		t.Fatalf("written config is not valid: %v", err)
	}
}

func TestIsDefaultConfigMissing(t *testing.T) {
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, ".config", "tuicast", "config.yaml")

	if !isDefaultConfigMissing(defaultPath, os.ErrNotExist) {
		t.Error("expected true for default path with ErrNotExist")
	}

	if isDefaultConfigMissing("/custom/path.yaml", os.ErrNotExist) {
		t.Error("expected false for non-default path")
	}

	if isDefaultConfigMissing("", os.ErrNotExist) {
		t.Error("expected false for empty path")
	}
}
