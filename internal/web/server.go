package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/party"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Size limits for generated and pasted puzzles.
const (
	MinN     = 4
	MaxN     = 15
	DefaultN = 8
)

// SolveFunc runs the solver on a puzzle. Cancelling ctx cancels the solve
// (solver.Options.Done); the error is then ctx's.
type SolveFunc func(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error)

// Budget limits how many solves run at once across everything of serve
// that solves: pages, the pool, the library and parties. The solver uses
// every core for its lookahead, so without a limit a crawler opening the
// library starts a solve per page at once.
//
// Solves nobody waits for yet (Background) take at most all slots but
// one, so a page that waits for its solve finds a slot even while a
// crawler has queued the whole library.
//
// A solve waits for its slots or for its ctx. Never solve while holding a
// slot of generator.cpuSlots, nor generate while holding a slot of the
// budget: generate first, then solve.
type Budget struct {
	slots      chan struct{} // every solve
	background chan struct{} // solves nobody waits for: one fewer, at least 1
}

// NewBudget returns a budget of n solves at once.
func NewBudget(n int) *Budget {
	return &Budget{slots: make(chan struct{}, n), background: make(chan struct{}, max(1, n-1))}
}

// SolveSlots is the n of NewBudget for serve: as many solves as the
// generator runs attempts at once (generator.cpuSlots), one core left for
// the rest.
func SolveSlots() int { return max(1, runtime.GOMAXPROCS(0)-1) }

// Limit returns solve within b, for solves a request waits for.
func (b *Budget) Limit(solve SolveFunc) SolveFunc {
	return func(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error) {
		select {
		case b.slots <- struct{}{}:
		case <-ctx.Done():
			return solver.Result{}, ctx.Err()
		}
		defer func() { <-b.slots }()
		return solve(ctx, p)
	}
}

// Background returns solve within b, for solves nobody waits for yet: the
// pool, a party's next puzzles, a hard library puzzle (its page comes
// without the result). It takes a background slot first, then a slot.
func (b *Budget) Background(solve SolveFunc) SolveFunc {
	limited := b.Limit(solve)
	return func(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error) {
		select {
		case b.background <- struct{}{}:
		case <-ctx.Done():
			return solver.Result{}, ctx.Err()
		}
		defer func() { <-b.background }()
		return limited(ctx, p)
	}
}

// Handler serves the interactive page with a form to generate new puzzles.
// Every page has a stable URL, so reloading keeps the puzzle (and,
// via the page's local storage, the progress):
//
//	/?n=8&seed=42   generated puzzle (same seeding as `starbattle generate`)
//	/               the unsolved solo puzzle of the cookie SoloCookie, else
//	                as /?n=8
//	/?n=8           new seed, redirects to the URL above
//	/?n=6&n=8&n=10  a random one of these sizes, then as /?n=8
//	/play?p=AAB...  puzzle in the text format, e.g. from the designer (rows
//	                separated by newlines, '/' or ',')
//
// With a pool (may be nil), /?n=8 hands out the seed of the puzzle the pool
// has ready, so the page appears without waiting for the generator. The
// last pages of /?n=…&seed=… and /play are kept (results), so a reload or
// a shared link answers at once. With
// parties (may be nil), /party and below serve the party mode (party_server.go).
// With libraries (may be none), /bibliothek serves them (library.go). With a
// symbol catalog (may be nil), /einstellungen lets players choose the symbol
// of their stars (symbols.go). /anleitung is the tutorial (tutorial.go).
// /manifest.webmanifest and the icons make the site installable (install.go).
// Pages and JSON go out gzipped to browsers that accept it (gzip.go).
func Handler(solve SolveFunc, pool *Pool, parties *party.Manager, libs []*Library, syms *Symbols) http.Handler {
	mux := http.NewServeMux()
	cache := newResults(cachedResults)
	// page solves p for request r, keeps the result under key and writes
	// the page; nothing if r is gone.
	page := func(w http.ResponseWriter, r *http.Request, key, title string, p *puzzle.Puzzle, sol []int) {
		res, err := solve(r.Context(), p)
		if r.Context().Err() != nil {
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		cache.put(key, &served{p, sol, res})
		write(w, title, p, sol, res)
	}
	if parties != nil {
		partyRoutes(mux, parties)
	}
	if len(libs) > 0 {
		libraryRoutes(mux, libs)
	}
	if syms != nil {
		symbolRoutes(mux, syms)
	}
	tutorialRoutes(mux, solve)
	installRoutes(mux)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "" {
			if u := soloURL(r); u != "" {
				http.Redirect(w, r, u, http.StatusFound)
				return
			}
		}
		q := r.URL.Query()
		sizes := []int{}
		for _, s := range q["n"] {
			v, err := strconv.Atoi(s)
			if err != nil || v < MinN || v > MaxN {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("Die Größe muss zwischen %d und %d liegen.", MinN, MaxN))
				return
			}
			if !slices.Contains(sizes, v) {
				sizes = append(sizes, v)
			}
		}
		if len(sizes) == 0 {
			sizes = append(sizes, DefaultN)
		}
		n := sizes[rand.IntN(len(sizes))]
		seedStr := q.Get("seed")
		if seedStr == "" {
			seed, ok := pool.Take(n)
			if !ok {
				seed = newSeed()
			}
			http.Redirect(w, r, fmt.Sprintf("/?n=%d&seed=%d", n, seed), http.StatusFound)
			return
		}
		if len(sizes) > 1 {
			writeError(w, http.StatusBadRequest, "Zu einer Puzzle-Nummer gehört genau eine Größe.")
			return
		}
		seed, err := strconv.ParseUint(seedStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Diese Puzzle-Nummer ist ungültig.")
			return
		}
		title := fmt.Sprintf("Star Battle %d×%d · Nr. %d", n, n, seed)
		key := fmt.Sprintf("%d/%d", n, seed)
		if e := pool.Lookup(n, seed); e != nil {
			// Kept, as the pool keeps only the last one per size.
			cache.put(key, &served{e.p, e.sol, e.res})
			write(w, title, e.p, e.sol, e.res)
			return
		}
		if v := cache.get(key); v != nil {
			write(w, title, v.p, v.sol, v.res)
			return
		}
		p, sol, err := generate(r.Context().Done(), n, seed)
		if err != nil {
			if r.Context().Err() == nil {
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		page(w, r, key, title, p, sol)
	})
	mux.HandleFunc("GET /play", func(w http.ResponseWriter, r *http.Request) {
		p, err := parsePlay(r.URL.Query().Get("p"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Puzzle nicht lesbar: "+err.Error())
			return
		}
		title := fmt.Sprintf("Star Battle %d×%d · eigenes Puzzle", p.N, p.N)
		key := "play:" + p.String() // the same puzzle however its link writes the rows
		if v := cache.get(key); v != nil {
			write(w, title, v.p, v.sol, v.res)
			return
		}
		var sol []int
		sols := bruteforce.Solve(p, 2)
		switch len(sols) {
		case 0:
			writeError(w, http.StatusBadRequest, "Das Puzzle hat keine Lösung.")
			return
		case 1:
			sol = sols[0]
		}
		page(w, r, key, title, p, sol)
	})
	return compress(mux)
}

// parsePlay reads the puzzle of /play?p=…: the text format, rows separated
// by newlines, '/', ',' or ';'.
func parsePlay(text string) (*puzzle.Puzzle, error) {
	text = strings.NewReplacer("/", "\n", ",", "\n", ";", "\n").Replace(text)
	p, err := puzzle.Parse(strings.NewReader(text))
	if err == nil && (p.N < MinN || p.N > MaxN) {
		//lint:ignore ST1005 German noun, shown to the user
		err = fmt.Errorf("Größe %d×%d, erlaubt sind %d bis %d", p.N, p.N, MinN, MaxN)
	}
	return p, err
}

// SoloCookie holds the URL of the solo puzzle "/" leads back to: Solo in the
// navigation, the start page, the start of the installed app (decision 27).
// The puzzle page sets it to its own URL while unsolved and removes it once
// solved (app.js); library puzzles, party rounds and the tutorial leave it.
const SoloCookie = "starbattle.solo"

// soloURL is the puzzle of the request's SoloCookie, "" if there is none.
// Only a puzzle URL of this site counts, rebuilt from its parts, so the
// cookie cannot send anyone to another site.
func soloURL(r *http.Request) string {
	c, err := r.Cookie(SoloCookie)
	if err != nil {
		return ""
	}
	v, err := url.QueryUnescape(c.Value)
	if err != nil {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return ""
	}
	q := u.Query()
	switch {
	case u.Path == "/" && len(q) == 2 && len(q["n"]) == 1 && len(q["seed"]) == 1:
		n, err := strconv.Atoi(q.Get("n"))
		if err != nil || n < MinN || n > MaxN {
			return ""
		}
		seed, err := strconv.ParseUint(q.Get("seed"), 10, 64)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("/?n=%d&seed=%d", n, seed)
	case u.Path == "/play" && len(q) == 1 && len(q["p"]) == 1:
		if _, err := parsePlay(q.Get("p")); err != nil {
			return ""
		}
		return "/play?" + url.Values{"p": q["p"]}.Encode()
	}
	return ""
}

// newSeed returns a random seed, short enough for the title and never 0 (the
// CLI's "time based").
func newSeed() uint64 { return uint64(rand.IntN(999_999) + 1) }

// generate gives the puzzle of /?n=n&seed=seed (same as `starbattle generate`).
// Closing done (may be nil) cancels it.
func generate(done <-chan struct{}, n int, seed uint64) (*puzzle.Puzzle, []int, error) {
	return generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{MinRegionSize: generator.DefaultMinRegionSize(n), Done: done})
}

// write writes the page for a solved puzzle, marked as served so it shows the forms.
func write(w http.ResponseWriter, title string, p *puzzle.Puzzle, sol []int, res solver.Result) {
	d := Build(title, p, sol, res)
	d.Served = true
	d.Text = p.String()
	writePage(w, d)
}

func writePage(w http.ResponseWriter, d Data) {
	if d.Served {
		d.MinN, d.MaxN = MinN, MaxN
	}
	writeHTML(w, http.StatusOK, func(b io.Writer) error { return Write(b, d) })
}

// writeError sends the error page with msg and the given links.
func writeError(w http.ResponseWriter, status int, msg string, links ...Link) {
	writeHTML(w, status, func(b io.Writer) error { return WriteError(b, "Star Battle · Fehler", msg, links...) })
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
