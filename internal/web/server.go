package web

import (
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Size limits for generated and pasted puzzles.
const (
	MinN     = 4
	MaxN     = 14
	DefaultN = 8
)

// SolveFunc runs the solver on a puzzle.
type SolveFunc func(p *puzzle.Puzzle) (solver.Result, error)

// Handler serves the interactive page with a form to generate new puzzles or
// paste one. Every page has a stable URL, so reloading keeps the puzzle (and,
// via the page's local storage, the progress):
//
//	/?n=8&seed=42   generated puzzle (same seeding as `starbattle generate`)
//	/?n=8           new seed, redirects to the URL above
//	/play?p=AAB...  pasted puzzle in the text format (rows separated by
//	                newlines, '/' or ',')
//
// With a pool (may be nil), /?n=8 hands out the seed of the puzzle the pool
// has ready, so the page appears without waiting for the generator. With
// parties (may be nil), /party and below serve the party mode (party_server.go).
// With a library (may be nil), /bibliothek serves it (library.go).
func Handler(solve SolveFunc, pool *Pool, parties *Parties, lib *Library) http.Handler {
	mux := http.NewServeMux()
	if parties != nil {
		partyRoutes(mux, parties)
	}
	if lib != nil {
		libraryRoutes(mux, lib)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		n := DefaultN
		if s := q.Get("n"); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil || v < MinN || v > MaxN {
				http.Error(w, fmt.Sprintf("n muss zwischen %d und %d liegen", MinN, MaxN), http.StatusBadRequest)
				return
			}
			n = v
		}
		seedStr := q.Get("seed")
		if seedStr == "" {
			seed, ok := pool.Take(n)
			if !ok {
				seed = newSeed()
			}
			http.Redirect(w, r, fmt.Sprintf("/?n=%d&seed=%d", n, seed), http.StatusFound)
			return
		}
		seed, err := strconv.ParseUint(seedStr, 10, 64)
		if err != nil {
			http.Error(w, "ungültiger Seed", http.StatusBadRequest)
			return
		}
		title := fmt.Sprintf("Star Battle %d×%d · Nr. %d", n, n, seed)
		if e := pool.Lookup(n, seed); e != nil {
			write(w, title, e.p, e.sol, e.res)
			return
		}
		p, sol, err := generate(n, seed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		page(w, solve, title, p, sol)
	})
	mux.HandleFunc("GET /play", func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("p")
		text = strings.NewReplacer("/", "\n", ",", "\n", ";", "\n").Replace(text)
		p, err := puzzle.Parse(strings.NewReader(text))
		if err == nil && (p.N < MinN || p.N > MaxN) {
			//lint:ignore ST1005 German noun, shown to the user
			err = fmt.Errorf("Größe %d×%d, erlaubt sind %d bis %d", p.N, p.N, MinN, MaxN)
		}
		if err != nil {
			http.Error(w, "Puzzle nicht lesbar: "+err.Error(), http.StatusBadRequest)
			return
		}
		var sol []int
		sols := bruteforce.Solve(p, 2)
		switch len(sols) {
		case 0:
			http.Error(w, "Das Puzzle hat keine Lösung.", http.StatusBadRequest)
			return
		case 1:
			sol = sols[0]
		}
		page(w, solve, fmt.Sprintf("Star Battle %d×%d · eigenes Puzzle", p.N, p.N), p, sol)
	})
	return mux
}

// newSeed returns a random seed, short enough for the title and never 0 (the
// CLI's "time based").
func newSeed() uint64 { return uint64(rand.IntN(999_999) + 1) }

// generate gives the puzzle of /?n=n&seed=seed (same as `starbattle generate`).
func generate(n int, seed uint64) (*puzzle.Puzzle, []int, error) {
	return generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{MinRegionSize: generator.DefaultMinRegionSize(n)})
}

// page solves p and writes the page.
func page(w http.ResponseWriter, solve SolveFunc, title string, p *puzzle.Puzzle, sol []int) {
	res, err := solve(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	write(w, title, p, sol, res)
}

// write writes the page for a solved puzzle, marked as served so it shows the forms.
func write(w http.ResponseWriter, title string, p *puzzle.Puzzle, sol []int, res solver.Result) {
	d := Build(title, p, sol, res)
	d.Served = true
	d.Text = p.String()
	writePage(w, d)
}

func writePage(w http.ResponseWriter, d Data) {
	writeHTML(w, http.StatusOK, func(b io.Writer) error { return Write(b, d) })
}

// writeHTML renders a page into a buffer first, so a rendering error still
// gets a proper error response, and sends it with the given status.
func writeHTML(w http.ResponseWriter, status int, render func(io.Writer) error) {
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	buf.WriteTo(w)
}
