package render

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// TraceOptions control Trace.
type TraceOptions struct {
	// Verbose explains every step in words, with the refuted chains of
	// assumptions nested below their step; otherwise one short line per step.
	Verbose bool
	// Boards draws the board after every step (new cells in brackets), and
	// after a refuted chain the board that shows its contradiction.
	Boards bool
	// NoColor prints region letters instead of colored backgrounds.
	NoColor bool
}

// Trace writes the steps of a solver's trace (starbattle solve -verbose,
// -step), replayed on a copy of start. Chains of assumptions are numbered
// within their step (8.1, 8.2, …) and drawn indented, before the step they
// explain.
func Trace(w io.Writer, start *board.State, trace []solver.Step, o TraceOptions) {
	steps(w, start.Clone(), trace, "", "", o)
}

func steps(w io.Writer, replay *board.State, trace []solver.Step, prefix, indent string, o TraceOptions) {
	for i, st := range trace {
		num := fmt.Sprintf("%s%d", prefix, i+1)
		// The chain explains this step, so print it before applying the step.
		before := replay.Clone()
		hl := map[puzzle.Pos]bool{}
		for _, m := range st.Marks {
			replay.Set(m.Pos.Row, m.Pos.Col, m.Kind) // conflicting last step: keep what fits
			hl[m.Pos] = true
		}
		if o.Verbose && st.Reason != "" {
			fmt.Fprintf(w, "%s\n%s%s. %s\n", strings.TrimRight(indent, " "), indent, num, st.Reason)
			if !o.Boards {
				if d := crosses(st.Marks); d != "" && st.Hypothesis == nil {
					fmt.Fprintf(w, "%s    %s\n", indent, d)
				}
			}
		} else {
			fmt.Fprintf(w, "%s\n%sstep %s: %s (%d cells)\n", strings.TrimRight(indent, " "), indent, num, st.Source, len(st.Marks))
		}
		if h := st.Hypothesis; h != nil && o.Verbose {
			trial := before
			steps(w, trial, h.Steps, num+".", indent+"    │ ", o)
			fmt.Fprintf(w, "%s    │ ⇒ Widerspruch: %v\n", indent, h.Contradiction)
			if o.Boards {
				conflict := map[puzzle.Pos]bool{}
				if ce, ok := errors.AsType[*board.ContradictionError](h.Contradiction); ok {
					for _, q := range ce.Cells {
						conflict[q] = true
					}
				}
				indented(w, State(trial, Options{NoColor: o.NoColor, Conflict: conflict}), indent+"    │ ")
			}
			fmt.Fprintf(w, "%s    ⇒ also ist (%d,%d) kein Stern\n", indent, st.Marks[0].Pos.Row, st.Marks[0].Pos.Col)
		}
		if o.Boards {
			indented(w, State(replay, Options{NoColor: o.NoColor, Highlight: hl}), indent)
		}
	}
}

// crosses lists the crosses of one step. Steps that place a star name it in
// their reason already, so they get no extra line.
func crosses(marks []solver.Mark) string {
	var cs []string
	for _, m := range marks {
		if m.Kind == board.Star {
			return ""
		}
		cs = append(cs, fmt.Sprintf("(%d,%d)", m.Pos.Row, m.Pos.Col))
	}
	if len(cs) == 0 {
		return ""
	}
	return "X: " + strings.Join(cs, " ")
}

// indented writes a drawn board with every line indented.
func indented(w io.Writer, b, indent string) {
	for _, line := range strings.SplitAfter(strings.TrimRight(b, "\n"), "\n") {
		fmt.Fprint(w, indent+line)
	}
	fmt.Fprintln(w)
}
