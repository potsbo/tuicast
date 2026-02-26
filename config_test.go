package main

import (
	"fmt"
	"testing"
)

func TestParseConfig_MinimalFormView(t *testing.T) {
	input := `
views:
  main:
    form:
      - name: file
        list: find . -type f
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
	if v.Form[0].List != "find . -type f" {
		t.Errorf("expected list 'find . -type f', got %q", v.Form[0].List)
	}
}

func TestParseConfig_FormStepWithDisplayAndPreview(t *testing.T) {
	input := `
views:
  main:
    form:
      - name: file
        list: find . -type f
        display: basename {}
        preview: head -50 {}
    run: vim $file
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if step.Display != "basename {}" {
		t.Errorf("expected display 'basename {}', got %q", step.Display)
	}
	if step.Preview != "head -50 {}" {
		t.Errorf("expected preview 'head -50 {}', got %q", step.Preview)
	}
}

func TestParseConfig_InputStep(t *testing.T) {
	input := `
views:
  main:
    form:
      - name: query
        placeholder: Enter search term
    run: grep -r $query .
`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	step := cfg.Views["main"].Form[0]
	if step.Placeholder != "Enter search term" {
		t.Errorf("expected placeholder 'Enter search term', got %q", step.Placeholder)
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

func TestParseConfig_FormStepMustHaveListOrPlaceholder(t *testing.T) {
	input := `
views:
  main:
    form:
      - name: x
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for step with neither list nor placeholder")
	}
}

func TestParseConfig_FormStepCannotHaveBothListAndPlaceholder(t *testing.T) {
	input := `
views:
  main:
    form:
      - name: x
        list: echo hello
        placeholder: Type something
    run: echo $x
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for step with both list and placeholder")
	}
}

func TestParseConfig_FormStepMustHaveName(t *testing.T) {
	input := `
views:
  main:
    form:
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
    form:
      - name: file
        list: find . -type f
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
    form:
      - name: file
        list: find . -type f
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
    form:
      - name: file
        list: find . -type f
    run: vim $file
  branches:
    form:
      - name: branch
        list: git branch --format=%(refname:short)
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

func TestParseConfig_UnionRefMustBeFormView(t *testing.T) {
	input := `
views:
  main:
    union: [sub]
  sub:
    menu: [leaf]
  leaf:
    run: echo hi
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for union referencing non-FormView")
	}
}

func TestParseConfig_UnionRefMustHaveExactlyOneStep(t *testing.T) {
	input := `
views:
  main:
    union: [multi]
  multi:
    form:
      - name: a
        list: echo a
      - name: b
        list: echo b
    run: echo $a $b
`
	_, err := ParseConfig([]byte(input))
	if err == nil {
		t.Fatal("expected error for union referencing FormView with 2 steps")
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
    form:
      - name: file
        list: find . -type f
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
