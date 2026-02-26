package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func shellOutput(cmd string, env []string) (string, error) {
	c := exec.Command("sh", "-c", cmd)
	c.Env = append(os.Environ(), env...)
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func shellLines(cmd string, env []string) ([]string, error) {
	out, err := shellOutput(cmd, env)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func shellExec(cmd string, env []string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Env = append(os.Environ(), env...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func historyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "tuicast", "history")
}

func executeView(cfg *Config, viewName string) error {
	v, ok := cfg.Views[viewName]
	if !ok {
		return fmt.Errorf("view %q not found", viewName)
	}

	switch {
	case v.Run != "":
		return executeFormView(cfg, viewName, &v)
	case len(v.Union) > 0:
		return executeUnionView(cfg, viewName, &v)
	case len(v.Menu) > 0:
		return executeMenuView(cfg, viewName, &v)
	default:
		return fmt.Errorf("view %q: unknown type", viewName)
	}
}

func executeFormView(cfg *Config, name string, v *View) error {
	env := []string{}

	for _, step := range v.Form {
		if step.List != "" {
			value, err := executeSelectStep(&step, env)
			if err != nil {
				return err
			}
			env = append(env, step.Name+"="+value)
		} else {
			value, err := fzfTextInput(step.Placeholder)
			if err != nil {
				return err
			}
			env = append(env, step.Name+"="+value)
		}
	}

	expanded, err := shellOutput("echo "+v.Run, env)
	if err != nil {
		expanded = v.Run
	}
	_ = appendHistory(historyPath(), expanded)

	return shellExec(v.Run, env)
}

func executeSelectStep(step *FormStep, env []string) (string, error) {
	lines, err := shellLines(step.List, env)
	if err != nil {
		return "", fmt.Errorf("list command failed: %w", err)
	}

	displayLines := lines
	if step.Display != "" {
		displayLines, err = transform(step.Display, lines, env)
		if err != nil {
			return "", fmt.Errorf("display transform failed: %w", err)
		}
	}

	opts := FzfOptions{}
	if step.Preview != "" {
		if strings.Contains(step.Preview, "{}") {
			opts.Preview = step.Preview
		}
	}

	result, err := fzfSelect(displayLines, opts)
	if err != nil {
		return "", err
	}

	if result.Index >= 0 && result.Index < len(lines) {
		return lines[result.Index], nil
	}
	return result.Line, nil
}

func executeUnionView(cfg *Config, name string, v *View) error {
	var allItems []string
	type itemMeta struct {
		viewName string
		rawLine  string
	}
	var metas []itemMeta

	for _, ref := range v.Union {
		refView := cfg.Views[ref]
		step := refView.Form[0]

		lines, err := shellLines(step.List, nil)
		if err != nil {
			return fmt.Errorf("view %q: list command failed: %w", ref, err)
		}

		displayLines := lines
		if step.Display != "" {
			displayLines, err = transform(step.Display, lines, nil)
			if err != nil {
				return fmt.Errorf("view %q: display transform failed: %w", ref, err)
			}
		}

		for i, line := range lines {
			display := displayLines[i]
			allItems = append(allItems, ref+"\t"+line+"\t"+display)
			metas = append(metas, itemMeta{viewName: ref, rawLine: line})
		}
	}

	previewScript := generatePreviewDispatcher(cfg, v.Union)

	opts := FzfOptions{
		Delimiter: "\t",
		WithNth:   "3",
	}
	if previewScript != "" {
		opts.Preview = previewScript
	}

	result, err := fzfSelect(allItems, opts)
	if err != nil {
		return err
	}

	if result.Index < 0 || result.Index >= len(metas) {
		return fmt.Errorf("invalid selection")
	}
	meta := metas[result.Index]

	refView := cfg.Views[meta.viewName]
	stepName := refView.Form[0].Name
	env := []string{stepName + "=" + meta.rawLine}

	expanded, err := shellOutput("echo "+refView.Run, env)
	if err != nil {
		expanded = refView.Run
	}
	_ = appendHistory(historyPath(), expanded)

	return shellExec(refView.Run, env)
}

func generatePreviewDispatcher(cfg *Config, refs []string) string {
	var cases []string
	hasPreview := false
	for _, ref := range refs {
		refView := cfg.Views[ref]
		step := refView.Form[0]
		if step.Preview != "" {
			hasPreview = true
			previewCmd := step.Preview
			if strings.Contains(previewCmd, "{}") {
				previewCmd = strings.ReplaceAll(previewCmd, "{}", "{2}")
			}
			cases = append(cases, fmt.Sprintf("  %s) %s ;;", ref, previewCmd))
		}
	}
	if !hasPreview {
		return ""
	}
	script := "case {1} in\n"
	script += strings.Join(cases, "\n")
	script += "\nesac"
	return script
}

func executeMenuView(cfg *Config, name string, v *View) error {
	selfBin, _ := os.Executable()

	var items []string
	for _, ref := range v.Menu {
		refView := cfg.Views[ref]
		title := refView.Title
		if title == "" {
			title = ref
		}
		items = append(items, ref+"\t"+title)
	}

	opts := FzfOptions{
		Delimiter: "\t",
		WithNth:   "2",
		Bind: []string{
			fmt.Sprintf("enter:execute(%s --view {1})+abort", selfBin),
		},
	}

	_, err := fzfSelect(items, opts)
	if err == ErrCancelled {
		return nil
	}
	return err
}
