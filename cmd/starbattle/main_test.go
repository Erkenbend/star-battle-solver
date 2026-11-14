package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Smoke tests for the commands: they run in-process with os.Stdin, os.Stdout
// and os.Stderr swapped for temporary files, so they must not run in
// parallel. main and usage are left out, they exit the process.

// run calls cmd with args and stdin as input and returns what it printed.
func run(t *testing.T, cmd func([]string) error, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "stdin")
	if err := os.WriteFile(in, []byte(stdin), 0o644); err != nil {
		t.Fatal(err)
	}
	files := make([]*os.File, 3)
	for i, name := range []string{"stdin", "stdout", "stderr"} {
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		files[i] = f
	}
	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = files[0], files[1], files[2]
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr
		for _, f := range files {
			f.Close()
		}
	}()
	err = cmd(args)
	out, _ := os.ReadFile(filepath.Join(dir, "stdout"))
	errOut, _ := os.ReadFile(filepath.Join(dir, "stderr"))
	return string(out), string(errOut), err
}

func mustContain(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("%s: missing %q in\n%s", what, want, got)
		}
	}
}

// writeFile writes text into a new file in a temporary directory.
func writeFile(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	// A unique solution (testdata of internal/board).
	unique5 = "DBBBB\nDDBCC\nDDBAA\nEDDAA\nEAAAA\n"
	// Four 2×2 squares: two solutions.
	twoSolutions = "AABB\nAABB\nCCDD\nCCDD\n"
	// Region A covers two whole rows: no solution.
	noSolution = "AAAA\nAAAA\nBBCC\nDDCC\n"
	// Needs lookahead even with all rules (4 steps).
	hard10 = "../../testdata/hard/n10_s453.txt"
)

func TestGenerate(t *testing.T) {
	out, errOut, err := run(t, cmdGenerate, "", "-n", "5", "-seed", "3", "-show", "-nocolor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "# seed 3\n") || strings.Count(out, "\n") != 6 {
		t.Errorf("stdout = %q", out)
	}
	mustContain(t, "stderr", errOut, "★", " A ")

	out, _, err = run(t, cmdGenerate, "", "-n", "10") // time-based seed, default minimum region size
	if err != nil || strings.Count(out, "\n") != 11 {
		t.Errorf("n=10: %v\n%s", err, out)
	}
	if _, _, err := run(t, cmdGenerate, "", "-n", "3"); err == nil {
		t.Error("n=3: expected error")
	}
}

func TestShow(t *testing.T) {
	for _, tt := range []struct {
		puzzle, want string
	}{
		{unique5, "unique solution:"},
		{twoSolutions, "WARNING: multiple solutions"},
		{noSolution, "no solution"},
	} {
		out, _, err := run(t, cmdShow, tt.puzzle, "-solution", "-nocolor") // from stdin
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, tt.want, out, tt.want)
	}
	out, _, err := run(t, cmdShow, "", writeFile(t, "p.txt", unique5))
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "colored", out, "\x1b[48;5;")
	if _, _, err := run(t, cmdShow, "", filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Error("missing file: expected error")
	}
	if _, _, err := run(t, cmdShow, "AB\n"); err == nil {
		t.Error("bad puzzle: expected error")
	}
}

func TestSolve(t *testing.T) {
	out, _, err := run(t, cmdSolve, unique5, "-nocolor")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "concurrent", out, "result: solved")

	out, _, err = run(t, cmdSolve, unique5, "-runner", "sync", "-verbose", "-nocolor")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "verbose", out, "1. ", "X: (", "result: solved")

	// Lookahead with nested chains, rendered step by step.
	out, _, err = run(t, cmdSolve, "", "-runner", "sync", "-search", "lookahead", "-step", "-verbose", "-nocolor", hard10)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "lookahead", out, "    │ ", "⇒ Widerspruch:", "kein Stern", "search: 4 lookahead crosses", "result: solved")

	// Without search and without the verbose text: stuck, step names only.
	out, _, err = run(t, cmdSolve, "", "-runner", "sync", "-search", "none", "-step", "-nocolor", hard10)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "no search", out, "step 1: ", "result: stuck")

	// No solution: the solver reports the contradiction.
	out, _, err = run(t, cmdSolve, noSolution, "-runner", "sync", "-nocolor")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "contradiction", out, "result: contradiction", "(")

	for _, args := range [][]string{{"-runner", "nope"}, {"-search", "nope"}} {
		if _, _, err := run(t, cmdSolve, unique5, args...); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
	if _, _, err := run(t, cmdSolve, "AB\n"); err == nil {
		t.Error("bad puzzle: expected error")
	}
}

func TestHTML(t *testing.T) {
	out, _, err := run(t, cmdHTML, unique5)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "stdout", out, "<!doctype html>", "Star Battle 5×5")

	path := filepath.Join(t.TempDir(), "page.html")
	if _, _, err := run(t, cmdHTML, "", "-o", path, writeFile(t, "herz.txt", unique5)); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "file", string(page), "Star Battle 5×5 · herz")

	if _, _, err := run(t, cmdHTML, unique5, "-o", filepath.Join(t.TempDir(), "no", "dir.html")); err == nil {
		t.Error("unwritable output: expected error")
	}
	if _, _, err := run(t, cmdHTML, unique5, "-runner", "nope"); err == nil {
		t.Error("bad runner: expected error")
	}
	if _, _, err := run(t, cmdHTML, "AB\n"); err == nil {
		t.Error("bad puzzle: expected error")
	}
}

// serve and design load everything and then fail to listen, so they return
// at once.
func TestServeAndDesignBadAddr(t *testing.T) {
	_, errOut, err := run(t, cmdServe, "", "-addr", "localhost:-1", "-no-pregen")
	if err == nil {
		t.Error("serve: expected listen error")
	}
	mustContain(t, "serve", errOut, "Star Battle auf http://localhost:-1/")

	_, errOut, err = run(t, cmdDesign, "", "-addr", "localhost:-1", "-dir", t.TempDir())
	if err == nil {
		t.Error("design: expected listen error")
	}
	mustContain(t, "design", errOut, "Designer auf http://localhost:-1/")
}

func TestHard(t *testing.T) {
	dir := t.TempDir()
	// Seed 453 is n10_s453.txt of the library.
	out, _, err := run(t, cmdHard, "", "-n", "10", "-from", "452", "-count", "2", "-o", dir)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "first search", out, "n=10 seed=453", "HIT → n10_s453.txt", "next search: -from 454", "append to bibliothek.idx")
	text, err := os.ReadFile(filepath.Join(dir, "n10_s453.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(hard10)
	if err != nil {
		t.Fatal(err)
	}
	header, body, _ := strings.Cut(string(text), "\n")
	mustContain(t, "header", header, "# generate -n 10 -seed 453 at commit ", "needs lookahead even with all rules (4 lookahead steps)")
	if !strings.HasSuffix(string(want), body) {
		t.Errorf("written puzzle differs from %s:\n%s", hard10, text)
	}

	out, _, err = run(t, cmdHard, "", "-n", "10", "-from", "453", "-count", "1", "-min-lookahead", "0", "-o", dir)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "second search", out, "(n10_s453.txt exists, kept)", "base rules stuck)")

	if _, _, err := run(t, cmdHard, "", "-n", "3"); err == nil {
		t.Error("n=3: expected error")
	}
}
