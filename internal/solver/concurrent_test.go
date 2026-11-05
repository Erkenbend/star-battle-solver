package solver

import (
	"fmt"
	"testing"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

// chainLink places the star of row r only once rows 0..r-1 have their stars, so
// solving needs N rounds of broadcasts between different goroutines.
type chainLink struct {
	row int
	sol []int
}

func (o chainLink) Name() string { return fmt.Sprintf("chain[%d]", o.row) }
func (o chainLink) Done(s *board.State) bool {
	return s.At(o.row, o.sol[o.row]) == board.Star
}
func (o chainLink) Deduce(s *board.State) []Mark {
	for r := range o.row {
		if s.At(r, o.sol[r]) != board.Star {
			return nil
		}
	}
	return []Mark{Star(puzzle.Pos{Row: o.row, Col: o.sol[o.row]}, "")}
}

// never deduces anything and is never done.
type never struct{}

func (never) Name() string               { return "never" }
func (never) Done(*board.State) bool     { return false }
func (never) Deduce(*board.State) []Mark { return nil }

func chain(sol []int, reverse bool) []Observer {
	obs := make([]Observer, len(sol))
	for r := range sol {
		i := r
		if reverse {
			i = len(sol) - 1 - r
		}
		obs[i] = chainLink{row: r, sol: sol}
	}
	return obs
}

func TestRunConcurrentChain(t *testing.T) {
	sol := []int{2, 4, 1, 3, 0}
	for i := range 200 {
		res := RunConcurrent(board.New(p5), append(chain(sol, i%2 == 1), never{}))
		if res.Status != Solved {
			t.Fatalf("run %d: status %v (%v)\n%s", i, res.Status, res.Err, res.State)
		}
	}
}

func TestRunConcurrentStuck(t *testing.T) {
	// Without the first link nothing can start: must terminate as stuck, not hang.
	sol := []int{2, 4, 1, 3, 0}
	for range 200 {
		res := RunConcurrent(board.New(p5), append(chain(sol, false)[1:], never{}, never{}))
		if res.Status != Stuck || len(res.Trace) != 0 {
			t.Fatalf("status %v, %d steps", res.Status, len(res.Trace))
		}
	}
	if res := RunConcurrent(board.New(p5), nil); res.Status != Stuck {
		t.Fatalf("no observers: status %v", res.Status)
	}
}

func TestRunConcurrentContradiction(t *testing.T) {
	fired := false
	bad := []Mark{Star(puzzle.Pos{Row: 0, Col: 0}, ""), Star(puzzle.Pos{Row: 1, Col: 1}, "")}
	res := RunConcurrent(board.New(p5), []Observer{scripted{bad, &fired}, never{}})
	if res.Status != Contradiction {
		t.Fatalf("status %v, want contradiction", res.Status)
	}
}

func TestRunConcurrentDoesNotModifyStart(t *testing.T) {
	start := board.New(p5)
	RunConcurrent(start, chain([]int{2, 4, 1, 3, 0}, false))
	if !start.Equal(board.New(p5)) {
		t.Fatal("start state was modified")
	}
}

// leveled emits one fixed mark at a priority level, optionally after a delay.
type leveled struct {
	mark  Mark
	level int
	delay time.Duration
}

func (o leveled) Name() string  { return fmt.Sprintf("leveled[%d]", o.level) }
func (o leveled) Priority() int { return o.level }
func (o leveled) Done(s *board.State) bool {
	return s.At(o.mark.Pos.Row, o.mark.Pos.Col) != board.Free
}
func (o leveled) Deduce(*board.State) []Mark {
	time.Sleep(o.delay)
	return []Mark{o.mark}
}

func TestRunConcurrentPriority(t *testing.T) {
	// The simple rule is slow, the demanding one answers at once; the simple
	// one must still be applied first. Both marks are compatible crosses.
	slow := leveled{Cross(puzzle.Pos{Row: 0, Col: 0}, ""), 1, 5 * time.Millisecond}
	fast := leveled{Cross(puzzle.Pos{Row: 4, Col: 4}, ""), 2, 0}
	for range 20 {
		for _, obs := range [][]Observer{{fast, slow}, {slow, fast}} {
			for name, run := range map[string]Runner{"sync": RunSync, "concurrent": RunConcurrent} {
				res := run(board.New(p5), obs)
				if len(res.Trace) != 2 || res.Trace[0].Level != 1 || res.Trace[1].Level != 2 {
					t.Fatalf("%s: trace %+v, want level 1 then level 2", name, res.Trace)
				}
			}
		}
	}
}
