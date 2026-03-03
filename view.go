package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func shellOutput(cmd string, env []string) (string, error) {
	c := exec.Command("sh", "-c", cmd)
	c.Env = append(os.Environ(), env...)
	out, err := c.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("command exited with %d: %s\n%s", exitErr.ExitCode(), cmd, strings.TrimSpace(string(exitErr.Stderr)))
		}
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
	shPath, err := exec.LookPath("sh")
	if err != nil {
		return err
	}

	script := cmd
	if os.Getenv("TMUX") != "" {
		escaped := strings.ReplaceAll(cmd, "'", "'\\''")
		const tmuxWrapper = `
# Save original stdout to fd 3, capture stderr into $__stderr
exec 3>&1
__stderr=$( (%s) 2>&1 1>&3 )
__rc=$?
exec 3>&-
# On failure, show error in a tmux popup
if [ $__rc -ne 0 ]; then
  tmux display-popup -e "TUICAST_ERR=error (exit $__rc): %s
$__stderr" -E 'echo "$TUICAST_ERR"; echo; echo "Press Enter to close..."; read _'
fi
exit $__rc
`
		script = fmt.Sprintf(tmuxWrapper, cmd, escaped)
	}

	environ := append(os.Environ(), env...)
	return syscall.Exec(shPath, []string{"sh", "-c", script}, environ)
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

type unionItem struct {
	fzfLine  string // view_name\traw\tdisplay for fzf
	viewName string
	rawLine  string
	isLeaf   bool // true = executeView directly (menu item), false = FormView with env
}

func collectUnionItems(cfg *Config, refs []string) ([]unionItem, error) {
	var items []unionItem
	for _, ref := range refs {
		target := cfg.Views[ref]
		switch {
		case target.isUnionView():
			inner, err := collectUnionItems(cfg, target.Union)
			if err != nil {
				return nil, err
			}
			items = append(items, inner...)
		case target.isMenuView():
			for _, menuRef := range target.Menu {
				menuView := cfg.Views[menuRef]
				title := menuView.Title
				if title == "" {
					title = menuRef
				}
				items = append(items, unionItem{
					fzfLine:  menuRef + "\t" + menuRef + "\t" + title,
					viewName: menuRef,
					isLeaf:   true,
				})
			}
		case target.isFormView() && len(target.Form) != 1:
			title := target.Title
			if title == "" {
				title = ref
			}
			items = append(items, unionItem{
				fzfLine:  ref + "\t" + ref + "\t" + title,
				viewName: ref,
				isLeaf:   true,
			})
		default:
			step := target.Form[0]
			lines, err := shellLines(step.List, nil)
			if err != nil {
				return nil, fmt.Errorf("view %q: list command failed: %w", ref, err)
			}
			displayLines := lines
			if step.Display != "" {
				displayLines, err = transform(step.Display, lines, nil)
				if err != nil {
					return nil, fmt.Errorf("view %q: display transform failed: %w", ref, err)
				}
			}
			for i, line := range lines {
				items = append(items, unionItem{
					fzfLine:  ref + "\t" + line + "\t" + displayLines[i],
					viewName: ref,
					rawLine:  line,
				})
			}
		}
	}
	return items, nil
}

func collectUnionFormRefs(cfg *Config, refs []string) []string {
	var result []string
	for _, ref := range refs {
		target := cfg.Views[ref]
		switch {
		case target.isUnionView():
			result = append(result, collectUnionFormRefs(cfg, target.Union)...)
		case target.isFormView() && len(target.Form) == 1:
			result = append(result, ref)
		}
	}
	return result
}

func executeUnionView(cfg *Config, name string, v *View) error {
	items, err := collectUnionItems(cfg, v.Union)
	if err != nil {
		return err
	}

	fzfLines := make([]string, len(items))
	for i, item := range items {
		fzfLines[i] = item.fzfLine
	}

	formRefs := collectUnionFormRefs(cfg, v.Union)
	previewScript := generatePreviewDispatcher(cfg, formRefs)

	opts := FzfOptions{
		Delimiter: "\t",
		WithNth:   "3",
	}
	if previewScript != "" {
		opts.Preview = previewScript
	}

	result, err := fzfSelect(fzfLines, opts)
	if err != nil {
		return err
	}

	if result.Index < 0 || result.Index >= len(items) {
		return fmt.Errorf("invalid selection")
	}
	selected := items[result.Index]

	if selected.isLeaf {
		return executeView(cfg, selected.viewName)
	}

	refView := cfg.Views[selected.viewName]
	stepName := refView.Form[0].Name
	env := []string{stepName + "=" + selected.rawLine}

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
	}

	for {
		result, err := fzfSelect(items, opts)
		if err != nil {
			return err
		}

		selected := strings.SplitN(items[result.Index], "\t", 2)[0]
		err = executeView(cfg, selected)
		if err == ErrCancelled {
			continue
		}
		return err
	}
}
