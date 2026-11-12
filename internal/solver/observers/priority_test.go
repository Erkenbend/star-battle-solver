package observers

import (
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// priorityStats counts the rule steps per level and the steps that broke the
// priority order (a simpler rule could already deduce something new).
type priorityStats struct {
	steps      map[int]int
	violations int
}

func (st priorityStats) String() string {
	var levels []int
	for l := range st.steps {
		levels = append(levels, l)
	}
	slices.Sort(levels)
	var parts []string
	for _, l := range levels {
		parts = append(parts, fmt.Sprintf("L%d:%d", l, st.steps[l]))
	}
	return fmt.Sprintf("%s, %d violations", strings.Join(parts, " "), st.violations)
}

// lowestApplicable returns the lowest level of an observer that would add a
// new mark to s, or -1 if none would.
func lowestApplicable(s *board.State, obs []solver.Observer) int {
	for _, o := range obs { // sorted by level
		if o.Done(s) {
			continue
		}
		for _, m := range o.Deduce(s) {
			if s.At(m.Pos.Row, m.Pos.Col) != m.Kind {
				return solver.PriorityOf(o)
			}
		}
	}
	return -1
}

// checkPriorities replays trace from start and records each rule step's level.
// A step that continues the previous step's observer is part of the same
// message (one Deduce call) and is not checked again.
func checkPriorities(start *board.State, obs []solver.Observer, trace []solver.Step, st *priorityStats) {
	s := start.Clone()
	for i, step := range trace {
		if step.Source != solver.HypothesisSource {
			st.steps[step.Level]++
			if i == 0 || trace[i-1].Source != step.Source {
				if low := lowestApplicable(s, obs); low >= 0 && low < step.Level {
					st.violations++
				}
			}
		}
		for _, m := range step.Marks {
			s.Set(m.Pos.Row, m.Pos.Col, m.Kind)
		}
	}
}

func priorityPuzzles(t *testing.T) []*puzzle.Puzzle {
	var ps []*puzzle.Puzzle
	for _, f := range hardTestFiles(t) {
		in, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		p, err := puzzle.Parse(in)
		in.Close()
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		ps = append(ps, p)
	}
	for n := 5; n <= 10; n++ {
		for seed := range uint64(8) {
			p, _, err := generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			ps = append(ps, p)
		}
	}
	return ps
}

// TestPriorityOrder: both runners only apply a rule when no simpler rule applies.
func TestPriorityOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	ps := priorityPuzzles(t)
	for _, r := range []struct {
		name string
		run  solver.Runner
	}{{"sync", solver.RunSync}, {"concurrent", solver.RunConcurrent}} {
		st := priorityStats{steps: map[int]int{}}
		for _, p := range ps {
			obs := All(p)
			res := r.run(board.New(p), obs)
			checkPriorities(board.New(p), obs, res.Trace, &st)
		}
		t.Logf("%s over %d puzzles: %v", r.name, len(ps), st)
		if st.violations > 0 {
			t.Errorf("%s: %d steps used a rule while a simpler one applied", r.name, st.violations)
		}
	}
}
