package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func transform(cmd string, lines []string, env []string) ([]string, error) {
	if strings.HasPrefix(cmd, "|") {
		return transformPipe(cmd[1:], lines, env)
	}
	return transformPerItem(cmd, lines, env)
}

func transformPerItem(cmd string, lines []string, env []string) ([]string, error) {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		expanded := strings.ReplaceAll(cmd, "{}", line)
		c := exec.Command("sh", "-c", expanded)
		c.Env = append(os.Environ(), env...)
		out, err := c.Output()
		if err != nil {
			return nil, err
		}
		result = append(result, strings.TrimRight(string(out), "\n"))
	}
	return result, nil
}

func transformPipe(cmd string, lines []string, env []string) ([]string, error) {
	pipeCmd := strings.TrimSpace(cmd)
	c := exec.Command("sh", "-c", pipeCmd)
	c.Env = append(os.Environ(), env...)
	c.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	out, err := c.Output()
	if err != nil {
		return nil, err
	}
	result := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(result) != len(lines) {
		return nil, fmt.Errorf("pipe command returned %d lines, expected %d", len(result), len(lines))
	}
	return result, nil
}
