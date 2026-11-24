// Package designer is the tool behind `starbattle design`, for the project's
// developers and deliberately not part of serve: draw part of a puzzle, let
// the generator complete it on all cores until it has several distinct
// completions, pick one, give its regions other colors if wanted and save it
// to a collection of the project (see
// package collection), usually designs/.
//
// The page is built like the others of internal/web (web.ServePage, with the
// shared style and logic.js), from its own assets/designer.html and
// assets/designer.js.
package designer

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/collection"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/web"
)

//go:embed assets/designer.html
var designerHTML string

//go:embed assets/designer.js
var designerJS string

// candidateChoices are the page's choices for how many completions to
// search; the search ends early if its attempts run out. The default is
// defaultCandidates.
var candidateChoices = []int{1, 2, 4, 8, 12, 16, 24}

const defaultCandidates = 8

// MaxTries bounds the generator attempts of one search, all completions
// together, for a pattern the generator rarely completes. On 16 cores they
// take about 10 s for an 8×8 drawing, 20 to 80 s for 10×10 to 14×14
// drawings; the page can stop earlier (decision 29).
const MaxTries = 100_000

// designAttempts bounds the attempts of one completion (one seed). It is
// not a limit of the search (MaxTries is) but part of what a seed means:
// only after this many attempts without may drawn regions grow (see
// generator.Complete), and the saved file names the seed.
const designAttempts = 300

// progressEvery is how often /api/complete reports its counters.
const progressEvery = 500 * time.Millisecond

// Page is the data of the designer page.
type Page struct {
	MinN       int    `json:"minN"`
	MaxN       int    `json:"maxN"`
	Dir        string `json:"dir"`
	Candidates int    `json:"candidates"` // default number of completions
	Choices    []int  `json:"choices"`    // see candidateChoices
	MaxTries   int    `json:"maxTries"`
}

// designRequest is the body of /api/check, /api/complete and /api/save.
type designRequest struct {
	Pattern    [][]int `json:"pattern"`              // region per cell, -1 = free
	Candidates int     `json:"candidates,omitempty"` // complete: how many to find at most
	Tries      int     `json:"tries,omitempty"`      // complete: attempts at most (default and at most MaxTries)
	Regions    [][]int `json:"regions,omitempty"`    // save: the chosen completion
	Seed       uint64  `json:"seed,omitempty"`       // save: its seed
	Colors     []int   `json:"colors,omitempty"`     // save: new region of each region of the completion (a permutation), none = as generated
	Name       string  `json:"name,omitempty"`       // save: file name (optional)
	Strict     bool    `json:"strict,omitempty"`     // drawn colors are final (generator.Options.Strict)
}

// patternProblem is the answer to /api/check, and part of /api/complete's
// answer if the pattern is not valid.
type patternProblem struct {
	Message string   `json:"message"`
	Cells   [][2]int `json:"cells,omitempty"`
}

// Candidate is one completion of a pattern.
type Candidate struct {
	Seed     uint64    `json:"seed"`
	Regions  [][]int   `json:"regions"`
	Solution []int     `json:"solution"`
	Status   string    `json:"status"`    // solver result, with search if the rules get stuck
	Steps    int       `json:"steps"`     // steps of the solver's trace
	RulesOut int       `json:"rulesFree"` // free cells when the rules alone got stuck, 0 if they solve it
	Stats    web.Stats `json:"stats"`
	Millis   int64     `json:"millis"` // time to generate
}

// designEvent is one line of /api/complete's answer (NDJSON): the pattern's
// problem (then nothing else follows), a candidate as soon as it is found
// and rated, the counters every progressEvery, or, last, done with the
// final counters.
type designEvent struct {
	Problem   *patternProblem `json:"problem,omitempty"`
	Candidate *Candidate      `json:"candidate,omitempty"`
	Progress  *designCounts   `json:"progress,omitempty"`
	Done      *designCounts   `json:"done,omitempty"`
}

// designCounts are the counters of a search.
type designCounts struct {
	Tried      int64 `json:"tried"`      // attempts of the generator
	Accepted   int64 `json:"accepted"`   // uniquely solvable new puzzles, at most k
	Duplicates int64 `json:"duplicates"` // puzzles found again (from another seed)
}

// Handler serves the designer. Saved puzzles go to dir. solve rates a
// completion. /play shows a puzzle to try it, as in serve, and /anleitung,
// where /play sends a new visitor first; with a symbol catalog (may be nil),
// its link to /einstellungen works too.
func Handler(dir string, solve web.SolveFunc, syms *web.Symbols) http.Handler {
	mux := http.NewServeMux()
	play := web.Handler(solve, nil, nil, nil, syms)
	mux.Handle("GET /play", play)
	mux.Handle("GET /anleitung", play)
	if syms != nil {
		for _, p := range []string{"GET /einstellungen", "GET /symbole.json", "GET /symbole/{file}"} {
			mux.Handle(p, play)
		}
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		d := Page{MinN: web.MinN, MaxN: web.MaxN, Dir: dir, Candidates: defaultCandidates, Choices: candidateChoices, MaxTries: MaxTries}
		web.ServePage(w, designerHTML, "Star Battle · Designer", d, designerJS)
	})
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
		req, ok := readDesignRequest(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, checkPattern(req.Pattern, req.Strict))
	})
	mux.HandleFunc("POST /api/complete", func(w http.ResponseWriter, r *http.Request) {
		req, ok := readDesignRequest(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-store")
		rc := http.NewResponseController(w)
		enc := json.NewEncoder(w)
		send := func(ev designEvent) {
			enc.Encode(ev) // one line per event
			rc.Flush()
		}
		if prob := checkPattern(req.Pattern, req.Strict); prob != nil {
			send(designEvent{Problem: prob})
			return
		}
		k := min(max(req.Candidates, 1), slices.Max(candidateChoices))
		tries := req.Tries
		if tries <= 0 || tries > MaxTries {
			tries = MaxTries
		}
		s := &search{pattern: req.Pattern, strict: req.Strict, k: int64(k), tries: int64(tries), solve: solve}
		out := s.run(r.Context())
		tick := time.NewTicker(progressEvery)
		defer tick.Stop()
		for {
			select {
			case c, ok := <-out:
				if !ok {
					send(designEvent{Done: s.counts()})
					return
				}
				send(designEvent{Candidate: c})
			case <-tick.C:
				send(designEvent{Progress: s.counts()})
			}
		}
	})
	mux.HandleFunc("POST /api/save", func(w http.ResponseWriter, r *http.Request) {
		req, ok := readDesignRequest(w, r)
		if !ok {
			return
		}
		name, err := saveDesign(dir, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"file": name})
	})
	return mux
}

func readDesignRequest(w http.ResponseWriter, r *http.Request) (designRequest, bool) {
	var req designRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "Anfrage nicht lesbar: "+err.Error(), http.StatusBadRequest)
		return req, false
	}
	n := len(req.Pattern)
	if n < web.MinN || n > web.MaxN {
		http.Error(w, fmt.Sprintf("Die Größe muss zwischen %d und %d liegen.", web.MinN, web.MaxN), http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// checkPattern runs generator.CheckPattern and explains its finding in German,
// or returns nil if the pattern is fine.
func checkPattern(pattern [][]int, strict bool) *patternProblem {
	n := len(pattern)
	minSize := generator.DefaultMinRegionSize(n)
	err := generator.CheckPattern(pattern, minSize, strict)
	if err == nil {
		return nil
	}
	pe, ok := errors.AsType[*generator.PatternError](err)
	if !ok {
		return &patternProblem{Message: "Ungültiges Muster: " + err.Error()}
	}
	p := &patternProblem{}
	for _, q := range pe.Cells {
		p.Cells = append(p.Cells, [2]int{q.Row, q.Col})
	}
	color := ""
	if pe.Region >= 0 {
		color = "Farbe " + string(puzzle.RegionLetter(pe.Region))
	}
	switch pe.Problem {
	case generator.Unreachable:
		if strict {
			p.Message = color + ": Der markierte Teil hängt nicht mit dem Rest der Farbe zusammen. Im strengen Modus bleibt jede Farbe genau so, wie sie gezeichnet ist, also muss sie zusammenhängend gezeichnet sein."
		} else {
			p.Message = color + ": Der markierte Teil lässt sich über freie Felder nicht mit dem Rest der Farbe verbinden. Jede Farbe muss am Ende eine zusammenhängende Fläche sein."
		}
	case generator.TooSmall:
		switch {
		case pe.Region < 0:
			p.Message = fmt.Sprintf("Die markierte freie Fläche hat nur %d Feld(er). Im strengen Modus füllen sie nur nicht gezeichnete Farben, und bei %d×%d braucht jede Farbe mindestens %d Felder.", len(pe.Cells), n, n, minSize)
		case strict:
			p.Message = fmt.Sprintf("%s hat nur %d Feld(er). Im strengen Modus wächst sie nicht mehr, und bei %d×%d braucht jede Farbe mindestens %d Felder.", color, len(pe.Cells), n, n, minSize)
		default:
			p.Message = fmt.Sprintf("%s ist ringsum eingeschlossen und hat nur %d Feld(er). Bei %d×%d braucht jede Farbe mindestens %d Felder.", color, len(pe.Cells), n, n, minSize)
		}
	case generator.Leftover:
		p.Message = fmt.Sprintf("Im strengen Modus bekommen freie Felder nur Farben, die nicht gezeichnet sind. Es gibt %d getrennte freie Flächen, aber nur noch %d ungenutzte Farbe(n). Freie Flächen schließen oder Farben wieder entfernen hilft.", pe.Areas, pe.Missing)
	case generator.Fixed:
		p.Message = "Im strengen Modus steht das Puzzle hier schon fest: Jede freie Fläche bekommt genau eine der übrigen Farben. So hat es aber mehrere Lösungen (markiert: wo sich zwei davon unterscheiden). Etwas mehr frei lassen oder eine Farbe weniger zeichnen gibt dem Generator Spielraum."
	case generator.Ambiguous:
		p.Message = "Die Sterne lassen sich allein auf gezeichneten Feldern auf mehrere Arten verteilen (markiert: wo sich zwei solche Verteilungen unterscheiden). Gezeichnete Felder ändert der Generator nie, also hätte jedes fertige Puzzle mehrere Lösungen. Einige Felder wieder freigeben hilft."
	default:
		p.Message = "Zu diesem Muster passt keine Verteilung der Sterne (je ein Stern pro Zeile, Spalte und Farbe, ohne Berührung). Das passiert z. B., wenn eine Farbe zwei ganze Zeilen füllt oder zwei eingeschlossene Farben nur in derselben Zeile Platz für ihren Stern haben."
		if strict {
			p.Message += " Im strengen Modus muss außerdem der Stern jeder gezeichneten Farbe auf ihren eigenen Feldern liegen."
		}
	}
	return p
}

// search completes a pattern again and again, each time with a new seed,
// until it has k distinct puzzles, tries attempts are used up or the request
// is gone. The work is the generator's: brute force decides uniqueness in
// every repair step. The rule-based solver only rates the k puzzles found.
type search struct {
	pattern [][]int
	strict  bool
	k       int64
	tries   int64
	solve   web.SolveFunc
	cancel  context.CancelFunc // ends the search: k found or tries used up

	tried, accepted, duplicates atomic.Int64

	mu    sync.Mutex
	seeds map[uint64]bool // seeds drawn so far
	seen  map[string]bool // puzzles found so far (collection.ID)
}

// run starts one worker per core but one (the generator's own CPU limit,
// shared with every other caller, applies to their attempts as well) and
// returns the channel on which they send the candidates, at most k. The
// channel is closed when the last worker has stopped; the caller must
// receive until then.
func (s *search) run(ctx context.Context) <-chan *Candidate {
	s.seeds, s.seen = map[uint64]bool{}, map[string]bool{}
	ctx, s.cancel = context.WithCancel(ctx)
	out := make(chan *Candidate)
	var wg sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)-1) {
		wg.Go(func() { s.work(ctx, out) })
	}
	go func() {
		wg.Wait()
		s.cancel()
		close(out)
	}()
	return out
}

// work completes the pattern with fresh seeds until ctx is done. The
// attempt that uses up the budget, or the k-th puzzle found, cancels ctx,
// which stops the attempts of the other workers too.
func (s *search) work(ctx context.Context, out chan<- *Candidate) {
	n := len(s.pattern)
	tried := func() {
		if s.tried.Add(1) >= s.tries {
			s.cancel()
		}
	}
	for ctx.Err() == nil {
		seed := s.newSeed()
		start := time.Now()
		p, sol, err := generator.Complete(s.pattern, generator.Seeded(n, seed), generator.Options{
			MinRegionSize: generator.DefaultMinRegionSize(n),
			MaxAttempts:   designAttempts,
			Workers:       1, // the workers are the parallelism
			Strict:        s.strict,
			Done:          ctx.Done(),
			Tried:         tried,
		})
		if err != nil || !s.accept(p) {
			continue // no puzzle from this seed, or cancelled, a duplicate, or k found already
		}
		c := &Candidate{Seed: seed, Regions: p.Regions, Solution: sol, Millis: time.Since(start).Milliseconds()}
		// Not ctx: the k-th puzzle cancels it, and its rating is wanted.
		if res, err := s.solve(context.Background(), p); err == nil {
			c.Status = res.Status.String()
			c.Steps = len(res.Trace)
			c.Stats = web.Stats{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses}
			c.RulesOut = freeWhenStuck(p, res.Trace)
		}
		out <- c
	}
}

// newSeed draws a seed not drawn before in this search: a seed always gives
// the same puzzle. Short, as in serve; it is written into the saved file.
func (s *search) newSeed() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		seed := uint64(rand.IntN(999_999) + 1)
		if !s.seeds[seed] {
			s.seeds[seed] = true
			return seed
		}
	}
}

// accept reports whether p is a new puzzle and one of the first k, counting
// it as accepted or as a duplicate. The k-th ends the search. Puzzles that
// other workers find while it ends are neither.
func (s *search) accept(p *puzzle.Puzzle) bool {
	id := collection.ID(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.accepted.Load() >= s.k:
		return false
	case s.seen[id]:
		s.duplicates.Add(1)
		return false
	}
	s.seen[id] = true
	if s.accepted.Add(1) == s.k {
		s.cancel()
	}
	return true
}

func (s *search) counts() *designCounts {
	return &designCounts{Tried: s.tried.Load(), Accepted: s.accepted.Load(), Duplicates: s.duplicates.Load()}
}

// freeWhenStuck replays trace up to its first search step and returns the
// free cells left at that point: how far the rules alone get. 0 if the
// trace needs no search.
func freeWhenStuck(p *puzzle.Puzzle, trace []solver.Step) int {
	s := board.New(p)
	for _, st := range trace {
		if st.Source == solver.HypothesisSource {
			free := 0
			for r := range p.N {
				for c := range p.N {
					if s.At(r, c) == board.Free {
						free++
					}
				}
			}
			return free
		}
		for _, m := range st.Marks {
			s.Set(m.Pos.Row, m.Pos.Col, m.Kind)
		}
	}
	return 0
}

var designName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`)

// saveDesign checks the chosen completion (valid, unique, contains the
// pattern, and with req.Strict no drawn region larger than drawn), gives its
// regions the colors of req.Colors if there are any, and writes it to dir as
// nNN_name.txt, with its name for the library
// (collection.NameKey, made from the file name), the seed and the pattern in
// comments, and appends it to the collection's index. Without a name, the
// next free nNN_dNNN is used. An existing file is never overwritten, and a name is used
// only once across sizes (the library tells motifs apart by name). Returns
// the file name.
func saveDesign(dir string, req designRequest) (string, error) {
	n := len(req.Pattern)
	if len(req.Regions) != n {
		return "", errors.New("Puzzle und Muster sind verschieden groß.")
	}
	for _, row := range req.Regions {
		if len(row) != n {
			return "", errors.New("Puzzle ist nicht quadratisch.")
		}
	}
	drawn := make([]bool, n)
	for _, row := range req.Pattern {
		if len(row) != n {
			return "", errors.New("Muster ist nicht quadratisch.")
		}
		for _, reg := range row {
			if reg != generator.Free {
				drawn[reg] = true
			}
		}
	}
	for r, row := range req.Pattern {
		for c, reg := range row {
			got := req.Regions[r][c]
			if reg != generator.Free && reg != got {
				return "", fmt.Errorf("Feld (%d,%d) passt nicht zum Muster.", r, c)
			}
			if reg == generator.Free && req.Strict && got >= 0 && got < n && drawn[got] {
				return "", fmt.Errorf("Feld (%d,%d): Im strengen Modus darf eine gezeichnete Farbe nicht wachsen.", r, c)
			}
		}
	}
	regions := req.Regions
	if req.Colors != nil {
		if !isPermutation(req.Colors, n) {
			return "", errors.New("Die Farben sind nicht vollständig verteilt.")
		}
		regions = make([][]int, n)
		for r, row := range req.Regions {
			for _, reg := range row {
				if reg < 0 || reg >= n {
					return "", fmt.Errorf("Puzzle ungültig: Region %d in Zeile %d.", reg, r)
				}
				regions[r] = append(regions[r], req.Colors[reg])
			}
		}
	}
	p, err := puzzle.New(regions)
	if err != nil {
		return "", fmt.Errorf("Puzzle ungültig: %v", err)
	}
	if bruteforce.CountSolutions(p, 2) != 1 {
		return "", errors.New("Das Puzzle hat keine eindeutige Lösung.")
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if name != "" && !designName.MatchString(name) {
		return "", errors.New("Name: nur a–z, 0–9, - und _ (höchstens 41 Zeichen).")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if name == "" {
		for i := 1; ; i++ {
			name = fmt.Sprintf("d%03d", i)
			if !collection.Exists(dir, collection.FileName(n, name)) {
				break
			}
		}
	}
	file := collection.FileName(n, name)
	if prev, err := collection.Find(os.DirFS(dir), ".", p); err != nil || prev != "" {
		if err == nil {
			err = fmt.Errorf("Dieses Puzzle ist schon als %s gespeichert.", prev)
		}
		return "", err
	}
	if same, err := collection.WithStem(dir, name); err != nil || (same != "" && same != file) {
		if err == nil {
			err = fmt.Errorf("Den Namen hat schon %s.", same)
		}
		return "", err
	}

	var b strings.Builder
	b.WriteString(collection.HeaderLine(collection.NameKey, collection.NameFromFile(file)))
	mode := ""
	if req.Strict {
		mode = ", Strict"
	}
	// The seeding of generator.Seeded, spelled out.
	fmt.Fprintf(&b, "# designer: generator.Complete(pattern, rand.NewPCG(%d, %d)%s), %s\n", req.Seed, n, mode, time.Now().Format("2006-01-02"))
	b.WriteString("# pattern (. = free):\n")
	for _, row := range req.Pattern {
		b.WriteString("#   ")
		for _, reg := range row {
			if reg == generator.Free {
				b.WriteByte('.')
			} else {
				b.WriteByte(puzzle.RegionLetter(reg))
			}
		}
		b.WriteByte('\n')
	}
	if req.Colors != nil {
		// The pattern above is what the generator was given; the puzzle below
		// has the colors chosen afterwards.
		b.WriteString("# colors swapped afterwards: ")
		for reg := range n {
			b.WriteByte(puzzle.RegionLetter(reg))
		}
		b.WriteString(" -> ")
		for _, to := range req.Colors {
			b.WriteByte(puzzle.RegionLetter(to))
		}
		b.WriteByte('\n')
	}
	b.WriteString(p.String())

	// Create never overwrites: two saves with the same name must not
	// overwrite each other.
	err = collection.Create(dir, file, b.String())
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%s gibt es schon.", file)
	}
	if err != nil {
		return "", err
	}
	return file, collection.AppendIndex(dir, file, motifIndexHeader)
}

// isPermutation reports whether colors holds each of 0..n-1 exactly once.
func isPermutation(colors []int, n int) bool {
	if len(colors) != n {
		return false
	}
	seen := make([]bool, n)
	for _, c := range colors {
		if c < 0 || c >= n || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}

// motifIndexHeader starts a new index of motifs.
const motifIndexHeader = `# Bibliothek, Motive: Reihenfolge der Puzzles je Größe. Der Designer hängt neue unten an.
# Nur unten anhängen, nie umsortieren: Die Übersicht zeigt sie je Größe in dieser Reihenfolge.
# Gespeichert und verlinkt wird über eine ID aus den Regionen; der Name steht in der Datei („# name:“).
`

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
