// Command starbattle generates and displays 1-star Star Battle puzzles.
//
//	starbattle generate -n 8 [-seed 42] [-show]   > p.txt
//	starbattle show [-solution] [-nocolor] [p.txt]   (reads stdin without file)
//	starbattle solve [-step] [-nocolor] [p.txt]
package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
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
  starbattle solve [-step] [-nocolor] [file]`)
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
	rng := rand.New(rand.NewPCG(s, s>>1|1))
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
	step := fs.Bool("step", false, "render the board after every observer step")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	fs.Parse(args)

	p, err := readPuzzle(fs)
	if err != nil {
		return err
	}
	o := render.Options{NoColor: *noColor}
	fmt.Print(render.Puzzle(p, o))

	res := solver.RunSync(board.New(p), observers.All(p))

	if *step {
		replay := board.New(p)
		for i, st := range res.Trace {
			hl := map[puzzle.Pos]bool{}
			for _, m := range st.Marks {
				replay.Set(m.Pos.Row, m.Pos.Col, m.Kind) // conflicting last step: keep what fits
				hl[m.Pos] = true
			}
			fmt.Printf("\nstep %d: %s (%d cells)\n", i+1, st.Source, len(st.Marks))
			fmt.Print(render.State(replay, render.Options{NoColor: *noColor, Highlight: hl}))
		}
	}

	fmt.Printf("\nresult: %v after %d steps", res.Status, len(res.Trace))
	if res.Err != nil {
		fmt.Printf(" (%v)", res.Err)
	}
	fmt.Println()
	fmt.Print(render.State(res.State, o))
	return nil
}
