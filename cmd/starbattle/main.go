// Command starbattle generates and displays 1-star Star Battle puzzles.
//
//	starbattle generate -n 8 [-seed 42] [-show]   > p.txt
//	starbattle show [-solution] [-nocolor] [p.txt]   (reads stdin without file)
//	starbattle solve [-runner concurrent|sync] [-search full|lookahead|none] [-step] [-verbose] [-nocolor] [p.txt]
package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/render"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "show":
		err = cmdShow(os.Args[2:])
	case "solve":
		err = cmdSolve(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  starbattle generate -n N [-seed S] [-show]
  starbattle show [-solution] [-nocolor] [file]
  starbattle solve [-runner concurrent|sync] [-search full|lookahead|none] [-step] [-verbose] [-nocolor] [file]`)
	os.Exit(2)
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	n := fs.Int("n", 8, "grid size")
	seed := fs.Uint64("seed", 0, "random seed (0 = time based)")
	show := fs.Bool("show", false, "also render puzzle and solution to stderr")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	fs.Parse(args)

	s := *seed
	if s == 0 {
		s = uint64(time.Now().UnixNano())
	}
	rng := rand.New(rand.NewPCG(s, uint64(*n))) // same seeding as the tests and testdata/hard
	p, sol, err := generator.Generate(*n, rng, generator.Options{})
	if err != nil {
		return err
	}
	fmt.Printf("# seed %d\n%s", s, p)
	if *show {
		o := render.Options{NoColor: *noColor}
		fmt.Fprintln(os.Stderr, render.Puzzle(p, o))
		fmt.Fprintln(os.Stderr, render.State(board.FromSolution(p, sol), o))
	}
	return nil
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	withSol := fs.Bool("solution", false, "also show the brute-force solution(s)")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	fs.Parse(args)

	p, err := readPuzzle(fs)
	if err != nil {
		return err
	}
	o := render.Options{NoColor: *noColor}
	fmt.Print(render.Puzzle(p, o))
	if *withSol {
		sols := bruteforce.Solve(p, 2)
		switch len(sols) {
		case 0:
			fmt.Println("\nno solution")
		case 1:
			fmt.Println("\nunique solution:")
		default:
			fmt.Println("\nWARNING: multiple solutions, showing one:")
		}
		if len(sols) > 0 {
			fmt.Print(render.State(board.FromSolution(p, sols[0]), o))
		}
	}
	return nil
}

func readPuzzle(fs *flag.FlagSet) (*puzzle.Puzzle, error) {
	var in io.Reader = os.Stdin
	if fs.NArg() > 0 {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		in = f
	}
	return puzzle.Parse(in)
}

func cmdSolve(args []string) error {
	fs := flag.NewFlagSet("solve", flag.ExitOnError)
	step := fs.Bool("step", false, "render the board after every deduction")
	verbose := fs.Bool("verbose", false, "explain every deduction in words")
	runner := fs.String("runner", "concurrent", "concurrent (one goroutine per observer) or sync")
	search := fs.String("search", "full", "when the rules get stuck: full (lookahead + backtracking), lookahead, or none")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	fs.Parse(args)

	p, err := readPuzzle(fs)
	if err != nil {
		return err
	}
	o := render.Options{NoColor: *noColor}
	fmt.Print(render.Puzzle(p, o))

	opts := solver.Options{Run: solver.RunConcurrent}
	switch *runner {
	case "concurrent":
	case "sync":
		opts.Run = solver.RunSync
	default:
		return fmt.Errorf("unknown runner %q", *runner)
	}
	switch *search {
	case "full":
		opts.Lookahead, opts.Backtrack = true, true
	case "lookahead":
		opts.Lookahead = true
	case "none":
	default:
		return fmt.Errorf("unknown search %q", *search)
	}
	res := solver.Solve(board.New(p), observers.All(p), opts)

	if *step || *verbose {
		replay := board.New(p)
		for i, st := range res.Trace {
			hl := map[puzzle.Pos]bool{}
			for _, m := range st.Marks {
				replay.Set(m.Pos.Row, m.Pos.Col, m.Kind) // conflicting last step: keep what fits
				hl[m.Pos] = true
			}
			if *verbose && st.Reason != "" {
				fmt.Printf("\n%d. %s\n", i+1, st.Reason)
				if !*step { // with -step the board highlights the cells instead
					fmt.Printf("    %s\n", describeMarks(st.Marks))
				}
			} else {
				fmt.Printf("\nstep %d: %s (%d cells)\n", i+1, st.Source, len(st.Marks))
			}
			if *step {
				fmt.Print(render.State(replay, render.Options{NoColor: *noColor, Highlight: hl}))
			}
		}
	}

	fmt.Printf("\nresult: %v after %d steps", res.Status, len(res.Trace))
	if res.Err != nil {
		fmt.Printf(" (%v)", res.Err)
	}
	if st := res.Stats; st != (solver.Stats{}) {
		fmt.Printf("\nsearch: %d lookahead crosses, %d guesses (%d refuted), max depth %d",
			st.Lookahead, st.Guesses, st.Refuted, st.MaxDepth)
	}
	fmt.Println()
	fmt.Print(render.State(res.State, o))
	return nil
}

// describeMarks lists the cells of one step: the stars (their implied crosses are
// only counted), or else all crosses.
func describeMarks(marks []solver.Mark) string {
	var stars, crosses []string
	for _, m := range marks {
		pos := fmt.Sprintf("(%d,%d)", m.Pos.Row, m.Pos.Col)
		if m.Kind == board.Star {
			stars = append(stars, pos)
		} else {
			crosses = append(crosses, pos)
		}
	}
	if len(stars) > 0 {
		out := "★ " + strings.Join(stars, " ")
		if len(crosses) > 0 {
			out += fmt.Sprintf(", dazu %d × X in Zeile, Spalte, Farbe und Nachbarschaft", len(crosses))
		}
		return out
	}
	if len(crosses) == 0 {
		return ""
	}
	return "X: " + strings.Join(crosses, " ")
}
