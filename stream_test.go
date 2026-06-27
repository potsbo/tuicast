package main

import (
	"bufio"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// collectLines reads all lines written to a pipe by fn, which receives the
// writer and must close it when done.
func collectLines(fn func(w *io.PipeWriter)) []string {
	pr, pw := io.Pipe()
	go fn(pw)
	var lines []string
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	pr.Close()
	return lines
}

func TestStreamListSourceItemsRaw(t *testing.T) {
	lines := collectLines(func(pw *io.PipeWriter) {
		streamListSourceItems(0, Source{List: "printf 'a\\nb'"}, pw, nil)
		pw.Close()
	})
	want := []string{"0\ta\ta", "0\tb\tb"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestStreamListSourceItemsPerItemDisplay(t *testing.T) {
	lines := collectLines(func(pw *io.PipeWriter) {
		streamListSourceItems(2, Source{List: "printf 'x\\ny'", Display: "printf 'D-%s' {}"}, pw, nil)
		pw.Close()
	})
	want := []string{"2\tx\tD-x", "2\ty\tD-y"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want[i])
		}
	}
}

// TestStreamListSourcesConcurrentNoInterleave runs two sources concurrently
// into a shared writer (as executeMultiSourceStep does) and asserts every line
// arrives intact and untangled, regardless of arrival order.
func TestStreamListSourcesConcurrentNoInterleave(t *testing.T) {
	sources := []Source{
		{List: "printf '0a\\n0b\\n0c'"},
		{List: "printf '1a\\n1b\\n1c'"},
	}
	lines := collectLines(func(pw *io.PipeWriter) {
		var wg sync.WaitGroup
		for srcIdx, src := range sources {
			wg.Add(1)
			go func(srcIdx int, src Source) {
				defer wg.Done()
				streamListSourceItems(srcIdx, src, pw, nil)
			}(srcIdx, src)
		}
		wg.Wait()
		pw.Close()
	})

	got := append([]string(nil), lines...)
	sort.Strings(got)
	want := []string{"0\t0a\t0a", "0\t0b\t0b", "0\t0c\t0c", "1\t1a\t1a", "1\t1b\t1b", "1\t1c\t1c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// Each emitted line must be a complete, well-formed 3-field record.
	for _, l := range lines {
		if strings.Count(l, "\t") != 2 {
			t.Errorf("corrupted/interleaved line: %q", l)
		}
	}
}

// TestStreamListSourcesFastNotBlockedBySlow asserts a fast source's items are
// observable before a slow source has finished — i.e. results stream rather
// than waiting for the slowest source.
func TestStreamListSourcesFastNotBlockedBySlow(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		var wg sync.WaitGroup
		// Slow source: sleeps before producing anything.
		wg.Add(1)
		go func() {
			defer wg.Done()
			streamListSourceItems(0, Source{List: "sleep 1; printf 'slow'"}, pw, nil)
		}()
		// Fast source: emits immediately.
		wg.Add(1)
		go func() {
			defer wg.Done()
			streamListSourceItems(1, Source{List: "printf 'fast'"}, pw, nil)
		}()
		wg.Wait()
		pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	start := time.Now()
	if !scanner.Scan() {
		t.Fatal("expected at least one line")
	}
	elapsed := time.Since(start)
	first := scanner.Text()
	pr.Close()

	if !strings.Contains(first, "fast") {
		t.Errorf("expected fast source first, got %q", first)
	}
	if elapsed > 700*time.Millisecond {
		t.Errorf("first line took %v; fast source appears blocked by slow source", elapsed)
	}
}
