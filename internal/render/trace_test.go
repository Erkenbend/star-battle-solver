package render

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

// hardTrace solves a puzzle the basic rules get stuck on, with lookahead, so
// the trace has refuted chains of assumptions.
func hardTrace(t *testing.T) (*board.State, []solver.Step) {
	t.Helper()
	text, err := os.ReadFile("../../testdata/hard/n05_s029.txt")
	if err != nil {
		t.Fatal(err)
	}
	p := puzzle.MustParse(string(text))
	res := solver.Solve(board.New(p), observers.AllWith(p, observers.BaseConfig), solver.Options{Lookahead: true})
	if res.Status != solver.Solved || res.Stats.Lookahead == 0 {
		t.Fatalf("want a solution with lookahead, got %v (%+v)", res.Status, res.Stats)
	}
	return board.New(p), res.Trace
}

func TestTraceVerbose(t *testing.T) {
	start, trace := hardTrace(t)
	before := start.Clone()
	var b strings.Builder
	Trace(&b, start, trace, TraceOptions{Verbose: true, NoColor: true})
	out := b.String()
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`(?m)^1\. \S`),                               // steps numbered, with their reason
		regexp.MustCompile(`(?m)^    │ \d+\.1\. \S`),                    // a chain nested below its step
		regexp.MustCompile(`(?m)^    │ ⇒ Widerspruch: \S`),              // ending in a contradiction
		regexp.MustCompile(`(?m)^    ⇒ also ist \(\d,\d\) kein Stern$`), // and its conclusion
		regexp.MustCompile(`(?m)^    X: \(\d,\d\)`),                     // the crosses of a step
	} {
		if !want.MatchString(out) {
			t.Errorf("no match for %s in\n%s", want, out)
		}
	}
	if strings.Count(out, "kein Stern") != countHypotheses(trace) {
		t.Errorf("%d conclusions for %d chains", strings.Count(out, "kein Stern"), countHypotheses(trace))
	}
	// The trace is replayed on a copy.
	if !start.Equal(before) {
		t.Error("Trace changed the start board")
	}
}

func TestTraceShortAndBoards(t *testing.T) {
	start, trace := hardTrace(t)
	var b strings.Builder
	Trace(&b, start, trace, TraceOptions{NoColor: true})
	out := b.String()
	if strings.Count(out, "\nstep ") != len(trace) || strings.Contains(out, "│") {
		t.Errorf("short form: want one line per step and no chains, got\n%s", out)
	}

	b.Reset()
	Trace(&b, start, trace, TraceOptions{Verbose: true, Boards: true, NoColor: true})
	out = b.String()
	if !strings.Contains(out, "[") || !strings.Contains(out, "│ ") {
		t.Errorf("boards: want highlighted cells and boards inside chains, got\n%s", out)
	}
	if strings.Contains(out, "    X: ") {
		t.Error("boards show the crosses, no extra X: line")
	}
}

func countHypotheses(trace []solver.Step) int {
	k := 0
	for _, st := range trace {
		if st.Hypothesis != nil {
			k++
		}
	}
	return k
}
