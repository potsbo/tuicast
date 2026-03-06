package main

import (
	"fmt"
	"testing"
)

func TestParseConfig_MinimalFormView(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: file
        sources:
          - list: find . -type f
    run: vim $file
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(cfg.Views))
	}
	v, ok := cfg.Views["main"]
	if !ok {
		t.Fatal("expected view 'main'")
	}
	if v.Run != "vim $file" {
		t.Errorf("expected run 'vim $file', got %q", v.Run)
	}
	if len(v.Form) != 1 {
		t.Fatalf("expected 1 form step, got %d", len(v.Form))
	}
	if v.Form[0].Name != "file" {
		t.Errorf("expected step name 'file', got %q", v.Form[0].Name)
	}
	if v.Form[0].Sources[0].List != "find . -type f" {
		t.Errorf("expected list 'find . -type f', got %q", v.Form[0].Sources[0].List)
	}
}

func TestParseConfig_FormStepWithDisplayAndPreview(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: file
        sources:
          - list: find . -type f
            display: basename {}
            preview: head -50 {}
    run: vim $file
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src := cfg.Views["main"].Form[0].Sources[0]
	if src.Display != "basename {}" {
		t.Errorf("expected display 'basename {}', got %q", src.Display)
	}
	if src.Preview != "head -50 {}" {
		t.Errorf("expected preview 'head -50 {}', got %q", src.Preview)
	}
}

func TestParseConfig_InputStep(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: query
        sources:
          - input: Enter search term
    run: grep -r $query .
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src := cfg.Views["main"].Form[0].Sources[0]
	if src.Input != "Enter search term" {
		t.Errorf("expected input 'Enter search term', got %q", src.Input)
	}
}

func TestParseConfig_ViewTypeMustBeExclusive(t *testing.T) {
	input := `
views:
  main:
    run: echo hello
    union: [foo]
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for view with both run and union")
	}
}

func TestParseConfig_ViewMustHaveType(t *testing.T) {
	input := `
views:
  main:
    title: Hello
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for view with no type")
	}
}

func TestParseConfig_FormStepMustHaveSources(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for step with no sources")
	}
}

func TestParseConfig_SourceMustHaveListOrInput(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources:
          - display: basename {}
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for source with neither list nor input")
	}
}

func TestParseConfig_SourceCannotHaveBothListAndInput(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources:
          - list: echo hello
            input: Type something
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for source with both list and input")
	}
}

func TestParseConfig_FormStepMustHaveName(t *testing.T) {
	input := `
views:
  main:
    steps:
      - sources:
          - list: echo hello
    run: echo done
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for step without name")
	}
}

func TestParseConfig_TransformCommandValid(t *testing.T) {
	tests := []struct {
		name    string
		display string
	}{
		{"per-item", "basename {}"},
		{"pipe", "| sed 's|.*/||'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
views:
  main:
    steps:
      - name: file
        sources:
          - list: find . -type f
            display: "%s"
    run: vim $file
`, tt.display)
			_, err := ParseConfig([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseConfig_TransformCommandInvalid(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: file
        sources:
          - list: find . -type f
            display: basename
    run: vim $file
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for display without {} or |")
	}
}

func TestParseConfig_UnionView(t *testing.T) {
	input := `
views:
  main:
    union: [files, branches]
  files:
    steps:
      - name: file
        sources:
          - list: find . -type f
    run: vim $file
  branches:
    steps:
      - name: branch
        sources:
          - list: git branch --format=%(refname:short)
    run: git switch $branch
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v := cfg.Views["main"]
	if len(v.Union) != 2 {
		t.Fatalf("expected 2 union refs, got %d", len(v.Union))
	}
	if v.Union[0] != "files" || v.Union[1] != "branches" {
		t.Errorf("unexpected union refs: %v", v.Union)
	}
}

func TestParseConfig_UnionRefMissing(t *testing.T) {
	input := `
views:
  main:
    union: [nonexistent]
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for union referencing nonexistent view")
	}
}

func TestParseConfig_UnionRefCanBeMenuView(t *testing.T) {
	input := `
views:
  main:
    union: [commands]
  commands:
    menu: [lazygit, nvim]
  lazygit:
    run: lazygit
  nvim:
    run: nvim
`
	_, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfig_UnionRefCanBeUnionView(t *testing.T) {
	input := `
views:
  main:
    union: [sessions, files]
  sessions:
    union: [tmux, zellij]
  tmux:
    steps:
      - name: s
        sources:
          - list: echo tmux-session
    run: tmux attach -t $s
  zellij:
    steps:
      - name: s
        sources:
          - list: echo zellij-session
    run: zellij attach $s
  files:
    steps:
      - name: file
        sources:
          - list: find . -type f
    run: vim $file
`
	_, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfig_UnionRefCanBeMultiStepFormView(t *testing.T) {
	input := `
views:
  main:
    union: [files, wizard]
  files:
    steps:
      - name: file
        sources:
          - list: echo file
    run: echo $file
  wizard:
    title: Wizard
    steps:
      - name: a
        sources:
          - list: echo a
      - name: b
        sources:
          - list: echo b
    run: echo $a $b
`
	_, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfig_MenuRefMissing(t *testing.T) {
	input := `
views:
  main:
    menu: [nonexistent]
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for menu referencing nonexistent view")
	}
}

func TestParseConfig_MenuView(t *testing.T) {
	input := `
views:
  main:
    menu: [files, lazygit]
  files:
    title: Files
    steps:
      - name: file
        sources:
          - list: find . -type f
    run: vim $file
  lazygit:
    title: Lazygit
    run: lazygit
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v := cfg.Views["main"]
	if len(v.Menu) != 2 {
		t.Fatalf("expected 2 menu refs, got %d", len(v.Menu))
	}
	if v.Menu[0] != "files" || v.Menu[1] != "lazygit" {
		t.Errorf("unexpected menu refs: %v", v.Menu)
	}
}

// New tests for sources features

func TestParseConfig_MultipleListSources(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: branch
        sources:
          - list: git branch --local
          - list: git branch --remote
    run: git switch $branch
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if len(step.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(step.Sources))
	}
	if step.Sources[0].List != "git branch --local" {
		t.Errorf("expected first list 'git branch --local', got %q", step.Sources[0].List)
	}
	if step.Sources[1].List != "git branch --remote" {
		t.Errorf("expected second list 'git branch --remote', got %q", step.Sources[1].List)
	}
}

func TestParseConfig_ComboboxListAndLabeledInput(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: branch
        sources:
          - list: git branch -a
          - label: "✨ Create new branch"
            input: Branch name
    run: git switch $branch
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if len(step.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(step.Sources))
	}
	if step.Sources[0].List != "git branch -a" {
		t.Errorf("expected list 'git branch -a', got %q", step.Sources[0].List)
	}
	if step.Sources[1].Input != "Branch name" {
		t.Errorf("expected input 'Branch name', got %q", step.Sources[1].Input)
	}
	if step.Sources[1].Label != "✨ Create new branch" {
		t.Errorf("expected label '✨ Create new branch', got %q", step.Sources[1].Label)
	}
}

func TestParseConfig_InputSourceCannotHaveDisplay(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources:
          - input: Enter value
            display: basename {}
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for input source with display")
	}
}

func TestParseConfig_InputSourceCannotHavePreview(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources:
          - input: Enter value
            preview: cat {}
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for input source with preview")
	}
}

func TestParseConfig_ListSourceCannotHaveLabel(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources:
          - list: echo hello
            label: "my label"
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for list source with label")
	}
}

func TestParseConfig_EmptySourcesArray(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: x
        sources: []
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for empty sources array")
	}
}

func TestParseConfig_IsInputOnly(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: query
        sources:
          - input: Search pattern
    run: grep -r $query .
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if !step.isInputOnly() {
		t.Error("expected isInputOnly() to be true for input-only step")
	}
}

func TestParseConfig_IsNotInputOnly(t *testing.T) {
	input := `
views:
  main:
    steps:
      - name: branch
        sources:
          - list: git branch -a
          - label: "✨ Create new branch"
            input: Branch name
    run: git switch $branch
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if step.isInputOnly() {
		t.Error("expected isInputOnly() to be false for combobox step")
	}
}
