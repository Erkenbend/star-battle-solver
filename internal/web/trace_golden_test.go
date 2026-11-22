package web

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/board"
	"starbattle/internal/collection"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

var update = flag.Bool("update", false, "rewrite the traces in testdata/traces")

// TestTraceGolden compares the trace the pages get for each puzzle of the
// sample (testdata/hard/sample.idx) with testdata/traces: every step's source,
// level, reason and marks in order, the refuted chains and their
// contradictions. A change to the search or the board that should not change
// any explanation keeps these files as they are; one that should (a new rule,
// different wording) rewrites them with -update and commits the diff. With
// -short without the 3-star puzzles, which take most of the time.
func TestTraceGolden(t *testing.T) {
	names, err := collection.ReadIndex(starbattle.Hard, starbattle.HardDir+"/sample.idx")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		text, err := starbattle.Hard.ReadFile(starbattle.HardDir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if testing.Short() && collection.Rating(text) >= 3 {
			continue
		}
		p, err := puzzle.Parse(bytes.NewReader(text))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// As serve and html solve: sync runner, lookahead and guessing, all rules.
		res := solver.Solve(board.New(p), observers.All(p), solver.Options{Run: solver.RunSync, Lookahead: true, Backtrack: true})
		var got bytes.Buffer
		writeTrace(&got, Build(name, p, nil, res))

		golden := filepath.Join("testdata", "traces", strings.TrimSuffix(name, ".txt")+".trace")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (new sample puzzle? go test ./internal/web -run TestTraceGolden -update)", err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			line, g, w := firstDiff(got.Bytes(), want)
			t.Errorf("%s: trace differs from %s at line %d:\n got: %s\nwant: %s\n(intended? go test ./internal/web -run TestTraceGolden -update, then review the diff)",
				name, golden, line, g, w)
		}
	}
}

// writeTrace writes the steps of d one per paragraph, refuted chains indented.
func writeTrace(w io.Writer, d Data) {
	fmt.Fprintf(w, "status: %s, lookahead %d, guesses %d\n", d.Status, d.Stats.Lookahead, d.Stats.Guesses)
	if d.Error != "" {
		fmt.Fprintf(w, "error: %s\n", d.Error)
	}
	writeSteps(w, d.Steps, "", "")
}

func writeSteps(w io.Writer, steps []Step, prefix, indent string) {
	for i, st := range steps {
		num := fmt.Sprintf("%s%d", prefix, i+1)
		fmt.Fprintf(w, "%s\n%s%s. %s (level %d)\n", strings.TrimRight(indent, " "), indent, num, st.Source, st.Level)
		if st.Reason != "" {
			fmt.Fprintf(w, "%s%s\n", indent, st.Reason)
		}
		marks := make([]string, len(st.Marks))
		for j, m := range st.Marks {
			marks[j] = fmt.Sprintf("%s(%d,%d)", map[int]string{0: "X", 1: "M"}[m[2]], m[0], m[1])
		}
		fmt.Fprintf(w, "%s%s\n", indent, strings.Join(marks, " "))
		if h := st.Hypothesis; h != nil {
			writeSteps(w, h.Steps, num+".", indent+"│ ")
			cells := make([]string, len(h.Cells))
			for j, q := range h.Cells {
				cells[j] = fmt.Sprintf("(%d,%d)", q[0], q[1])
			}
			fmt.Fprintf(w, "%s│ ⇒ %s: %s\n", indent, h.Contradiction, strings.Join(cells, " "))
		}
	}
}

// firstDiff returns the number and the two versions of the first line where a and b differ.
func firstDiff(a, b []byte) (int, string, string) {
	la, lb := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := range max(len(la), len(lb)) {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return i + 1, x, y
		}
	}
	return 0, "", ""
}
