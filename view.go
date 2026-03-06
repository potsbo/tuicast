package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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
	listCmd := exec.Command("sh", "-c", step.List)
	listCmd.Env = append(os.Environ(), env...)
	stdout, err := listCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("list command failed: %w", err)
	}
	if err := listCmd.Start(); err != nil {
		return "", fmt.Errorf("list command failed: %w", err)
	}

	opts := FzfOptions{}
	if step.Preview != "" {
		if strings.Contains(step.Preview, "{}") {
			opts.Preview = step.Preview
		}
	}

	var fzfInput io.Reader
	if step.Display == "" {
		fzfInput = stdout
	} else if strings.HasPrefix(step.Display, "|") {
		fzfInput, err = streamTransformPipe(stdout, step.Display[1:], env)
		if err != nil {
			listCmd.Wait()
			return "", fmt.Errorf("display transform failed: %w", err)
		}
		opts.Delimiter = "\t"
		opts.WithNth = "2"
	} else {
		fzfInput = streamTransformPerItem(stdout, step.Display, env)
		opts.Delimiter = "\t"
		opts.WithNth = "2"
	}

	if opts.Preview != "" && opts.Delimiter == "\t" {
		opts.Preview = strings.ReplaceAll(opts.Preview, "{}", "{1}")
	}

	result, err := fzfSelectStream(fzfInput, opts)
	listCmd.Wait()
	if err != nil {
		return "", err
	}

	selected := result.Line
	if opts.Delimiter == "\t" {
		selected = strings.SplitN(selected, "\t", 2)[0]
	}
	return selected, nil
}

func streamTransformPerItem(input io.Reader, cmd string, env []string) io.Reader {
	// Convert per-item transform into a single shell process.
	// e.g. "basename {}" → while IFS= read -r __l; do basename "$__l"; done
	pipeCmd := strings.ReplaceAll(cmd, "{}", "\"$__tuicast_line\"")
	script := fmt.Sprintf(`while IFS= read -r __tuicast_line; do __out=$(%s); printf '%%s\n' "$__out"; done`, pipeCmd)

	c := exec.Command("sh", "-c", script)
	c.Env = append(os.Environ(), env...)

	pipeIn, _ := c.StdinPipe()
	pipeOut, _ := c.StdoutPipe()
	c.Start()

	origCh := make(chan string, 64)
	pr, pw := io.Pipe()

	// Feed input lines to the transform shell.
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			origCh <- line
			fmt.Fprintln(pipeIn, line)
		}
		close(origCh)
		pipeIn.Close()
	}()

	// Zip originals with transform output, streaming each pair to fzf.
	go func() {
		dispScanner := bufio.NewScanner(pipeOut)
		for original := range origCh {
			if dispScanner.Scan() {
				fmt.Fprintf(pw, "%s\t%s\n", original, dispScanner.Text())
			}
		}
		c.Wait()
		pw.Close()
	}()

	return pr
}

func streamTransformPipe(input io.Reader, cmd string, env []string) (io.Reader, error) {
	pipeCmd := strings.TrimSpace(cmd)
	c := exec.Command("sh", "-c", pipeCmd)
	c.Env = append(os.Environ(), env...)

	pipeIn, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	pipeOut, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	// Read pipe command output concurrently to avoid OS pipe buffer deadlock.
	displayCh := make(chan []string, 1)
	go func() {
		var displays []string
		scanner := bufio.NewScanner(pipeOut)
		for scanner.Scan() {
			displays = append(displays, scanner.Text())
		}
		displayCh <- displays
	}()

	// Feed input to pipe command while collecting originals, then zip results.
	go func() {
		var originals []string
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			line := scanner.Text()
			originals = append(originals, line)
			fmt.Fprintln(pipeIn, line)
		}
		pipeIn.Close()

		displays := <-displayCh
		c.Wait()

		for i, original := range originals {
			if i < len(displays) {
				fmt.Fprintf(pw, "%s\t%s\n", original, displays[i])
			}
		}
		pw.Close()
	}()

	return pr, nil
}

func writeUnionItems(cfg *Config, refs []string, w io.Writer, seen map[string]bool) {
	for _, ref := range refs {
		target := cfg.Views[ref]
		switch {
		case target.isUnionView():
			writeUnionItems(cfg, target.Union, w, seen)
		case target.isMenuView():
			for _, menuRef := range target.Menu {
				if seen[menuRef] {
					continue
				}
				seen[menuRef] = true
				menuView := cfg.Views[menuRef]
				title := menuView.Title
				if title == "" {
					title = menuRef
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", menuRef, menuRef, title)
			}
		case target.isFormView() && len(target.Form) != 1:
			if seen[ref] {
				continue
			}
			seen[ref] = true
			title := target.Title
			if title == "" {
				title = ref
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", ref, ref, title)
		default:
			writeUnionFormItems(ref, &target, w, seen)
		}
	}
}

func writeUnionFormItems(ref string, target *View, w io.Writer, seen map[string]bool) {
	step := target.Form[0]
	listCmd := exec.Command("sh", "-c", step.List)
	stdout, err := listCmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := listCmd.Start(); err != nil {
		return
	}
	defer listCmd.Wait()

	if step.Display == "" {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if seen[line] {
				continue
			}
			seen[line] = true
			fmt.Fprintf(w, "%s\t%s\t%s\n", ref, line, line)
		}
	} else if strings.HasPrefix(step.Display, "|") {
		var lines []string
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if len(lines) == 0 {
			return
		}
		displays, err := transformPipe(step.Display[1:], lines, nil)
		if err != nil {
			return
		}
		for i, line := range lines {
			if i < len(displays) && !seen[line] {
				seen[line] = true
				fmt.Fprintf(w, "%s\t%s\t%s\n", ref, line, displays[i])
			}
		}
	} else {
		pipeCmd := strings.ReplaceAll(step.Display, "{}", "\"$__tuicast_line\"")
		script := fmt.Sprintf(`while IFS= read -r __tuicast_line; do __out=$(%s); printf '%%s\n' "$__out"; done`, pipeCmd)

		c := exec.Command("sh", "-c", script)
		pipeIn, err := c.StdinPipe()
		if err != nil {
			return
		}
		pipeOut, err := c.StdoutPipe()
		if err != nil {
			return
		}
		if err := c.Start(); err != nil {
			return
		}

		origCh := make(chan string, 64)
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				origCh <- line
				fmt.Fprintln(pipeIn, line)
			}
			close(origCh)
			pipeIn.Close()
		}()

		dispScanner := bufio.NewScanner(pipeOut)
		for original := range origCh {
			if dispScanner.Scan() {
				if !seen[original] {
					seen[original] = true
					fmt.Fprintf(w, "%s\t%s\t%s\n", ref, original, dispScanner.Text())
				}
			}
		}
		c.Wait()
	}
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
	pr, pw := io.Pipe()
	go func() {
		writeUnionItems(cfg, v.Union, pw, make(map[string]bool))
		pw.Close()
	}()

	formRefs := collectUnionFormRefs(cfg, v.Union)
	previewScript := generatePreviewDispatcher(cfg, formRefs)

	opts := FzfOptions{
		Delimiter: "\t",
		WithNth:   "3",
	}
	if previewScript != "" {
		opts.Preview = previewScript
	}

	result, err := fzfSelectStream(pr, opts)
	pr.Close()
	if err != nil {
		return err
	}

	parts := strings.SplitN(result.Line, "\t", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid selection")
	}
	selectedView := parts[0]
	selectedRaw := parts[1]

	refView := cfg.Views[selectedView]
	if refView.isFormView() && len(refView.Form) == 1 {
		stepName := refView.Form[0].Name
		env := []string{stepName + "=" + selectedRaw}

		expanded, err := shellOutput("echo "+refView.Run, env)
		if err != nil {
			expanded = refView.Run
		}
		_ = appendHistory(historyPath(), expanded)

		return shellExec(refView.Run, env)
	}

	return executeView(cfg, selectedView)
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
