package web

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

func load(t *testing.T, path string) *puzzle.Puzzle {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := puzzle.Parse(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return p
}

// build solves p like `starbattle html` does and returns the page data.
func build(p *puzzle.Puzzle) Data {
	res := solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true})
	var sol []int
	if sols := bruteforce.Solve(p, 2); len(sols) == 1 {
		sol = sols[0]
	}
	return Build("Test", p, sol, res)
}

func TestBuild(t *testing.T) {
	p := load(t, "../../testdata/hard/n11_s095.txt") // needs lookahead
	d := build(p)
	if d.N != 11 || len(d.Regions) != 11 || len(d.Letters) != 11 || len(d.Solution) != 11 {
		t.Fatalf("bad layout data: n=%d letters=%q solution=%v", d.N, d.Letters, d.Solution)
	}
	if d.Status != "solved" || len(d.Steps) == 0 {
		t.Fatalf("status %s with %d steps", d.Status, len(d.Steps))
	}
	chains := 0
	for _, st := range d.Steps {
		if st.Rule == "" || strings.Contains(st.Rule, "[") || st.Reason == "" || len(st.Marks) == 0 {
			t.Fatalf("incomplete step %+v", st)
		}
		if h := st.Hypothesis; h != nil {
			chains++
			if len(h.Steps) == 0 || h.Contradiction == "" || len(h.Cells) == 0 {
				t.Fatalf("incomplete chain %+v", h)
			}
			if first := h.Steps[0].Marks[0]; first[2] != 1 {
				t.Fatalf("chain starts with %v, want the assumed star", first)
			}
		}
	}
	if chains == 0 {
		t.Fatal("no lookahead chain in the data")
	}
}

func TestWrite(t *testing.T) {
	p := load(t, "../../testdata/hard/n05_s029.txt")
	d := build(p)
	d.Title = `<Test & "Titel">`
	var buf bytes.Buffer
	if err := Write(&buf, d); err != nil {
		t.Fatal(err)
	}
	page := buf.String()
	if strings.Contains(page, "{{") {
		t.Fatal("unreplaced placeholder")
	}
	if strings.Contains(page, d.Title) || !strings.Contains(page, "&lt;Test &amp; &#34;Titel&#34;&gt;") {
		t.Fatal("title not escaped")
	}
	// The embedded data parses back and contains no raw "</" that could end the script.
	m := regexp.MustCompile(`(?s)<script id="data" type="application/json">(.*?)</script>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no data script")
	}
	var back Data
	if err := json.Unmarshal([]byte(m[1]), &back); err != nil {
		t.Fatal(err)
	}
	if back.Title != d.Title || len(back.Steps) != len(d.Steps) {
		t.Fatalf("data did not round-trip: %q, %d steps", back.Title, len(back.Steps))
	}
	for _, needle := range []string{`id="board"`, "function boot()", ".cell.bt"} {
		if !strings.Contains(page, needle) {
			t.Errorf("page lacks %q", needle)
		}
	}
}

// TestJSLogic runs logic_test.js with node on the data of several puzzles:
// replaying the trace, following the hints to the solution, rule checks and
// the parsing of explanations.
func TestJSLogic(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	files, _ := filepath.Glob("../../testdata/hard/*.txt")
	if testing.Short() {
		files = files[:4]
	}
	dir := t.TempDir()
	args := []string{"logic_test.js"}
	for _, f := range files {
		data, err := json.Marshal(build(load(t, f)))
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, filepath.Base(f)+".json")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, out)
	}
	cmd := exec.Command(node, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Logf("%s", bytes.TrimSpace(out))
}
