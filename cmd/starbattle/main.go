// Command starbattle generates and displays 1-star Star Battle puzzles.
//
//	starbattle generate -n 8 [-seed 42] [-min-region K] [-show] [-nocolor]   > p.txt
//	starbattle show [-solution] [-nocolor] [p.txt]   (reads stdin without file)
//	starbattle solve [-runner concurrent|sync] [-search full|lookahead|none]
//	                 [-partner-depth N] [-dead-max N] [-pigeonhole-max N] [-non-contiguous]
//	                 [-step] [-verbose] [-nocolor] [p.txt]
//	starbattle html [-o page.html] [-title T] [solver flags] [p.txt]   interactive page
//	starbattle serve [-addr localhost:6369] [-no-pregen] [solver flags]   generate and solve in the browser
//	starbattle design [-addr localhost:6370] [-dir designs] [solver flags]   draw a pattern, complete it (for developers)
//	starbattle hard -n N [-from S] [-count C] [-min-lookahead K] [-o testdata/hard] [-workers W]   find hard puzzles (for developers)
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"starbattle"
	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/hardsearch"
	"starbattle/internal/puzzle"
	"starbattle/internal/render"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
	"starbattle/internal/web"
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
	case "html":
		err = cmdHTML(os.Args[2:])
	case "serve":
		err = cmdServe(os.Args[2:])
	case "design":
		err = cmdDesign(os.Args[2:])
	case "hard":
		err = cmdHard(os.Args[2:])
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
  starbattle generate -n N [-seed S] [-min-region K] [-show] [-nocolor]
  starbattle show [-solution] [-nocolor] [file]
  starbattle solve [-runner concurrent|sync] [-search full|lookahead|none]
                   [-partner-depth N] [-dead-max N] [-pigeonhole-max N] [-non-contiguous]
                   [-step] [-verbose] [-nocolor] [file]
  starbattle html [-o page.html] [-title T] [solver flags as for solve] [file]
  starbattle serve [-addr localhost:6369] [-no-pregen] [solver flags as for solve]
  starbattle design [-addr localhost:6370] [-dir designs] [solver flags as for solve]
  starbattle hard -n N [-from S] [-count C] [-min-lookahead K] [-o dir] [-workers W]`)
	os.Exit(2)
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	n := fs.Int("n", 8, "grid size")
	seed := fs.Uint64("seed", 0, "random seed (0 = time based)")
	show := fs.Bool("show", false, "also render puzzle and solution to stderr")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	minRegion := fs.Int("min-region", 0, "smallest allowed region size (0 = 2 from N = 10 on, else 1)")
	fs.Parse(args)

	s := *seed
	if s == 0 {
		s = uint64(time.Now().UnixNano())
	}
	rng := rand.New(rand.NewPCG(s, uint64(*n))) // same seeding as the tests and testdata/hard
	mr := *minRegion
	if mr == 0 {
		mr = generator.DefaultMinRegionSize(*n)
	}
	p, sol, err := generator.Generate(*n, rng, generator.Options{MinRegionSize: mr})
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

// solverFlags registers the flags that configure the solver and returns a
// function that runs it on p with those settings (call after fs.Parse).
func solverFlags(fs *flag.FlagSet, defaultRunner string) func(p *puzzle.Puzzle) (solver.Result, error) {
	runner := fs.String("runner", defaultRunner, "concurrent (one goroutine per observer) or sync")
	search := fs.String("search", "full", "when the rules get stuck: full (lookahead + backtracking), lookahead, or none")
	partnerDepth := fs.Int("partner-depth", observers.DefaultConfig.PartnerDepth, "ForcedPartner: max forced stars in a row (0 = off)")
	deadMax := fs.Int("dead-max", observers.DefaultConfig.DeadCellsMax, "DeadCells: max remaining cells of a unit (0 = off)")
	pigeonMax := fs.Int("pigeonhole-max", observers.DefaultConfig.PigeonholeMaxK, "Pigeonhole: max units checked at once after an assumed star (0 = off)")
	anySubset := fs.Bool("non-contiguous", observers.DefaultConfig.ConfinementAnySubset, "Confinement k≥2: any subset of rows/columns instead of only adjacent ones")
	return func(p *puzzle.Puzzle) (solver.Result, error) {
		opts := solver.Options{Run: solver.RunConcurrent}
		switch *runner {
		case "concurrent":
		case "sync":
			opts.Run = solver.RunSync
		default:
			return solver.Result{}, fmt.Errorf("unknown runner %q", *runner)
		}
		switch *search {
		case "full":
			opts.Lookahead, opts.Backtrack = true, true
		case "lookahead":
			opts.Lookahead = true
		case "none":
		default:
			return solver.Result{}, fmt.Errorf("unknown search %q", *search)
		}
		cfg := observers.Config{PartnerDepth: *partnerDepth, DeadCellsMax: *deadMax, ConfinementAnySubset: *anySubset, PigeonholeMaxK: *pigeonMax}
		return solver.Solve(board.New(p), observers.AllWith(p, cfg), opts), nil
	}
}

func cmdSolve(args []string) error {
	fs := flag.NewFlagSet("solve", flag.ExitOnError)
	step := fs.Bool("step", false, "render the board after every deduction")
	verbose := fs.Bool("verbose", false, "explain every deduction in words")
	noColor := fs.Bool("nocolor", false, "render with letters instead of colors")
	solve := solverFlags(fs, "concurrent")
	fs.Parse(args)

	p, err := readPuzzle(fs)
	if err != nil {
		return err
	}
	o := render.Options{NoColor: *noColor}
	fmt.Print(render.Puzzle(p, o))

	res, err := solve(p)
	if err != nil {
		return err
	}

	if *step || *verbose {
		printSteps(board.New(p), res.Trace, "", "", *step, *verbose, *noColor)
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

// cmdHTML writes an interactive page: solve the puzzle yourself (with checks
// and hints) or step through the solver's explanation.
func cmdHTML(args []string) error {
	fs := flag.NewFlagSet("html", flag.ExitOnError)
	out := fs.String("o", "", "output file (default: stdout)")
	title := fs.String("title", "", "page title (default: from the file name)")
	// sync by default: the step order (and so the explanation) is reproducible.
	solve := solverFlags(fs, "sync")
	fs.Parse(args)

	p, err := readPuzzle(fs)
	if err != nil {
		return err
	}
	res, err := solve(p)
	if err != nil {
		return err
	}
	var sol []int
	if sols := bruteforce.Solve(p, 2); len(sols) == 1 {
		sol = sols[0]
	}
	t := *title
	if t == "" {
		t = fmt.Sprintf("Star Battle %d×%d", p.N, p.N)
		if fs.NArg() > 0 {
			t += " · " + strings.TrimSuffix(filepath.Base(fs.Arg(0)), filepath.Ext(fs.Arg(0)))
		}
	}
	data := web.Build(t, p, sol, res)

	if *out == "" {
		return web.Write(os.Stdout, data)
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	// Close reports write errors the OS deferred, so it must not be dropped.
	return errors.Join(web.Write(f, data), f.Close())
}

// cmdServe serves the interactive page with a form to generate a new puzzle;
// each puzzle is solved on request. Unless -no-pregen is given, one
// puzzle of every size is generated ahead in the background.
// /party serves the party mode: playing the same puzzles together in rounds.
// /bibliothek offers the motifs of designs/ and the hard puzzles of
// testdata/hard (those to solve without help).
// /einstellungen lets players choose the symbol of their stars (symbols/).
func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "localhost:6369", "listen address")
	noPregen := fs.Bool("no-pregen", false, "do not keep a generated puzzle of every size ready")
	solve := solverFlags(fs, "sync") // sync: reproducible explanations, as for html
	fs.Parse(args)

	var pool *web.Pool
	if !*noPregen {
		var sizes []int
		for n := web.MinN; n <= web.MaxN; n++ {
			sizes = append(sizes, n)
		}
		pool = web.NewPool(solve, sizes)
	}
	// Ctrl-C or SIGTERM (systemd stop) cancels ctx. Request contexts derive
	// from it, so open event streams end too and Shutdown does not hang.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	motifs, err := web.LoadMotifs(starbattle.Designs, starbattle.DesignsDir, solve)
	if err != nil {
		return err
	}
	hard, err := web.LoadLibrary(starbattle.Hard, starbattle.HardDir, solve)
	if err != nil {
		return err
	}
	syms, err := web.LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		return err
	}
	parties := web.NewParties(solve, pool)
	go parties.Run(5*time.Second, ctx.Done())
	srv := &http.Server{
		Addr:              *addr,
		Handler:           web.Handler(solve, pool, parties, []*web.Library{motifs, hard}, syms),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "Star Battle auf http://%s/\n", *addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// cmdDesign serves the designer, a tool for developers of this project:
// draw part of a puzzle, let the generator complete it in several ways, pick
// one and save it to -dir. It is deliberately not part of serve.
func cmdDesign(args []string) error {
	fs := flag.NewFlagSet("design", flag.ExitOnError)
	addr := fs.String("addr", "localhost:6370", "listen address")
	dir := fs.String("dir", "designs", "directory for saved puzzles")
	solve := solverFlags(fs, "sync")
	fs.Parse(args)

	syms, err := web.LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           web.DesignerHandler(*dir, solve, syms),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "Designer auf http://%s/ (speichert nach %s)\n", *addr, *dir)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// cmdHard searches seeds for hard puzzles (see hardsearch), a tool for
// developers of this project to grow testdata/hard and so the library. One
// line per seed, in seed order; Ctrl-C stops after the running seeds and
// still prints where to continue.
func cmdHard(args []string) error {
	fs := flag.NewFlagSet("hard", flag.ExitOnError)
	n := fs.Int("n", 10, "grid size")
	from := fs.Uint64("from", 1, "first seed")
	count := fs.Uint64("count", 100, "number of seeds")
	minLookahead := fs.Int("min-lookahead", 1, "a hit needs at least this many lookahead steps with all rules (0: base rules stuck is enough)")
	out := fs.String("o", "", "write hits as nNN_sSEED.txt into this directory (e.g. testdata/hard)")
	workers := fs.Int("workers", 0, "seeds checked at the same time (0 = number of cores)")
	fs.Parse(args)
	if *n < 4 || *n > 52 {
		return fmt.Errorf("-n %d: must be 4 to 52", *n)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	commit := buildCommit()
	var stuck, hits int
	var written []string
	var firstErr error
	next := hardsearch.Search(ctx, hardsearch.Options{N: *n, From: *from, Count: *count, Workers: *workers}, func(f hardsearch.Found) {
		line := fmt.Sprintf("n=%d seed=%d", *n, f.Seed)
		if f.Err != nil {
			fmt.Printf("%s error: %v\n", line, f.Err)
			firstErr = cmp.Or(firstErr, fmt.Errorf("seed %d: %w", f.Seed, f.Err))
			return
		}
		v := f.Verdict
		line += fmt.Sprintf(" %v t=%v", v, f.Took.Round(time.Millisecond))
		if v.Base != solver.Solved {
			stuck++
		}
		if v.Hard(*minLookahead) {
			hits++
			line += "  HIT"
			if *out != "" {
				name := fmt.Sprintf("n%02d_s%03d.txt", *n, f.Seed)
				switch err := writeHard(filepath.Join(*out, name), *n, f, commit); {
				case errors.Is(err, os.ErrExist):
					line += " (" + name + " exists, kept)"
				case err != nil:
					line += " (" + err.Error() + ")"
					firstErr = cmp.Or(firstErr, err)
				default:
					line += " → " + name
					written = append(written, name)
				}
			}
		}
		fmt.Println(line)
	})

	fmt.Printf("\nseeds %d to %d: base rules stuck %d, hits %d (", *from, next-1, stuck, hits)
	if *minLookahead > 0 {
		fmt.Printf("all rules stuck, lookahead steps ≥ %d)\n", *minLookahead)
	} else {
		fmt.Printf("base rules stuck)\n")
	}
	if next < *from+*count {
		fmt.Printf("stopped early; continue with -from %d -count %d\n", next, *from+*count-next)
	} else {
		fmt.Printf("next search: -from %d\n", next)
	}
	if len(written) > 0 {
		fmt.Printf("written to %s; append to %s to put them in the library:\n", *out, web.LibraryIndex)
		for _, name := range written {
			fmt.Println(name)
		}
	}
	return firstErr
}

// writeHard writes f's puzzle to path, with a header that says how to make it
// again and why it is hard. An existing file is left alone (os.ErrExist).
func writeHard(path string, n int, f hardsearch.Found, commit string) error {
	v := f.Verdict
	why := "base rules get stuck, all rules solve it"
	if v.All == solver.Stuck {
		why = fmt.Sprintf("needs lookahead even with all rules (%d lookahead steps", v.Lookahead)
		if v.Guesses > 0 {
			why += fmt.Sprintf(", %d guesses", v.Guesses)
		}
		why += ")"
	}
	header := fmt.Sprintf("# generate -n %d -seed %d at commit %s (MinRegionSize %d); %s\n",
		n, f.Seed, commit, generator.DefaultMinRegionSize(n), why)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = io.WriteString(file, header+f.Puzzle.String())
	return errors.Join(err, file.Close())
}

// buildCommit is the commit the binary was built from, for the header of
// files written by hard: the generator's output depends on it. go run does
// not record it, go build does.
func buildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+changes"
			}
		}
	}
	if rev == "" {
		return "unknown"
	}
	return rev[:min(7, len(rev))] + dirty
}

// describeMarks lists the crosses of one step. Steps that place a star name it
// in their reason already, so they get no extra line.
func describeMarks(marks []solver.Mark) string {
	var crosses []string
	for _, m := range marks {
		if m.Kind == board.Star {
			return ""
		}
		crosses = append(crosses, fmt.Sprintf("(%d,%d)", m.Pos.Row, m.Pos.Col))
	}
	if len(crosses) == 0 {
		return ""
	}
	return "X: " + strings.Join(crosses, " ")
}

// printSteps replays trace on replay and prints each step; hypothesis chains are
// printed nested (numbered 8.1, 8.2, ...) on a scratch copy of the board.
func printSteps(replay *board.State, trace []solver.Step, prefix, indent string, step, verbose, noColor bool) {
	for i, st := range trace {
		num := fmt.Sprintf("%s%d", prefix, i+1)
		// The chain explains this step, so print it before applying the step.
		before := replay.Clone()
		hl := map[puzzle.Pos]bool{}
		for _, m := range st.Marks {
			replay.Set(m.Pos.Row, m.Pos.Col, m.Kind) // conflicting last step: keep what fits
			hl[m.Pos] = true
		}
		if verbose && st.Reason != "" {
			fmt.Printf("%s\n%s%s. %s\n", strings.TrimRight(indent, " "), indent, num, st.Reason)
			if !step {
				if d := describeMarks(st.Marks); d != "" && st.Hypothesis == nil {
					fmt.Printf("%s    %s\n", indent, d)
				}
			}
		} else {
			fmt.Printf("%s\n%sstep %s: %s (%d cells)\n", strings.TrimRight(indent, " "), indent, num, st.Source, len(st.Marks))
		}
		if h := st.Hypothesis; h != nil && verbose {
			trial := before
			printSteps(trial, h.Steps, num+".", indent+"    │ ", step, verbose, noColor)
			fmt.Printf("%s    │ ⇒ Widerspruch: %v\n", indent, h.Contradiction)
			if step {
				conflict := map[puzzle.Pos]bool{}
				if ce, ok := errors.AsType[*board.ContradictionError](h.Contradiction); ok {
					for _, q := range ce.Cells {
						conflict[q] = true
					}
				}
				printBoard(render.State(trial, render.Options{NoColor: noColor, Conflict: conflict}), indent+"    │ ")
			}
			fmt.Printf("%s    ⇒ also ist (%d,%d) kein Stern\n", indent, st.Marks[0].Pos.Row, st.Marks[0].Pos.Col)
		}
		if step {
			printBoard(render.State(replay, render.Options{NoColor: noColor, Highlight: hl}), indent)
		}
	}
}

func printBoard(b, indent string) {
	for _, line := range strings.SplitAfter(strings.TrimRight(b, "\n"), "\n") {
		fmt.Print(indent + line)
	}
	fmt.Println()
}
