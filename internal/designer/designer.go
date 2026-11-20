// Package designer is the tool behind `starbattle design`, for the project's
// developers and deliberately not part of serve: draw part of a puzzle, let
// the generator complete it again and again on all cores, keep the hardest
// completions, pick one and save it to a collection of the project (see
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

// candidateChoices are the page's choices for how many completions to show
// at most; the search finds many more and keeps the hardest. The default is
// defaultCandidates.
var candidateChoices = []int{1, 2, 4, 8, 12, 16, 24}

const defaultCandidates = 8

// MaxTries bounds the generator attempts of one search, all completions
// together. On 16 cores that is about 10 s for an 8×8 drawing, 20 to 80 s
// for 10×10 to 14×14 drawings, 12 minutes for an empty 14×14; the page can
// stop earlier (decision 29).
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
	Candidates int    `json:"candidates"` // default number of completions shown
	Choices    []int  `json:"choices"`    // see candidateChoices
	MaxTries   int    `json:"maxTries"`
}

// designRequest is the body of /api/check, /api/complete and /api/save.
type designRequest struct {
	Pattern    [][]int `json:"pattern"`              // region per cell, -1 = free
	Candidates int     `json:"candidates,omitempty"` // complete: how many to show at most
	Tries      int     `json:"tries,omitempty"`      // complete: attempts at most (default and at most MaxTries)
	Regions    [][]int `json:"regions,omitempty"`    // save: the chosen completion
	Seed       uint64  `json:"seed,omitempty"`       // save: its seed
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
	Millis   int64     `json:"millis"`  // time to generate
	Entered  int       `json:"entered"` // when it entered the k hardest: 1, 2, … (see harder)
}

// designEvent is one line of /api/complete's answer (NDJSON): the pattern's
// problem (then nothing else follows), a candidate as soon as it is among
// the k hardest so far, the counters every progressEvery, or, last, done
// with the final counters. A candidate can be pushed out of the k hardest
// by later ones; the page sorts them as harder does and shows the first k.
type designEvent struct {
	Problem   *patternProblem `json:"problem,omitempty"`
	Candidate *Candidate      `json:"candidate,omitempty"`
	Progress  *designCounts   `json:"progress,omitempty"`
	Done      *designCounts   `json:"done,omitempty"`
}

// designCounts are the counters of a search.
type designCounts struct {
	Tried      int64 `json:"tried"`      // attempts of the generator
	Accepted   int64 `json:"accepted"`   // uniquely solvable puzzles found, without duplicates
	Duplicates int64 `json:"duplicates"` // puzzles found again (from another seed)
}

// Handler serves the designer. Saved puzzles go to dir. solve rates a
// completion and serves /play, rules is the same solver without search: it
// rates every completion first, so the search runs only where the rules get
// stuck (see search.rate). /play shows a puzzle to try it, as in serve; with
// a symbol catalog (may be nil), its link to /einstellungen works too.
func Handler(dir string, solve, rules web.SolveFunc, syms *web.Symbols) http.Handler {
	mux := http.NewServeMux()
	play := web.Handler(solve, nil, nil, nil, syms)
	mux.Handle("GET /play", play)
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
		s := &search{pattern: req.Pattern, strict: req.Strict, k: k, tries: int64(tries), solve: solve, rules: rules}
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
// until tries attempts are used up or the request is gone, and keeps the k
// hardest distinct puzzles. The work is the generator's: brute force decides
// uniqueness in every repair step. The rule-based solver only rates what it
// finds, and its search only where that can change which puzzles are kept.
type search struct {
	pattern      [][]int
	strict       bool
	k            int
	tries        int64
	solve, rules web.SolveFunc

	tried, accepted, duplicates atomic.Int64

	mu      sync.Mutex
	seeds   map[uint64]bool // seeds drawn so far
	seen    map[string]bool // puzzles found so far (collection.ID)
	best    []*Candidate    // the hardest so far, hardest first, at most k
	entered int             // candidates that entered best so far
}

// run starts one worker per core but one (the generator's own CPU limit,
// shared with every other caller, applies to their attempts as well) and
// returns the channel on which they send each candidate that is among the k
// hardest when it is rated. The channel is closed when the last worker has
// stopped; the caller must receive until then.
func (s *search) run(ctx context.Context) <-chan *Candidate {
	s.seeds, s.seen = map[uint64]bool{}, map[string]bool{}
	ctx, cancel := context.WithCancel(ctx)
	out := make(chan *Candidate)
	var wg sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)-1) {
		wg.Go(func() { s.work(ctx, cancel, out) })
	}
	go func() {
		wg.Wait()
		cancel()
		close(out)
	}()
	return out
}

// work completes the pattern with fresh seeds until ctx is done. The
// attempt that reaches the budget cancels ctx, which stops the attempts of
// the other workers too.
func (s *search) work(ctx context.Context, cancel context.CancelFunc, out chan<- *Candidate) {
	n := len(s.pattern)
	tried := func() {
		if s.tried.Add(1) >= s.tries {
			cancel()
		}
	}
	for ctx.Err() == nil {
		seed := s.newSeed()
		start := time.Now()
		p, sol, err := generator.Complete(s.pattern, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{
			MinRegionSize: generator.DefaultMinRegionSize(n),
			MaxAttempts:   designAttempts,
			Workers:       1, // the workers are the parallelism
			Strict:        s.strict,
			Done:          ctx.Done(),
			Tried:         tried,
		})
		if err != nil || !s.fresh(p) {
			continue // no puzzle from this seed, or cancelled, or a duplicate
		}
		c := &Candidate{Seed: seed, Regions: p.Regions, Solution: sol, Millis: time.Since(start).Milliseconds()}
		if s.rate(p, c) {
			out <- c
		}
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

// fresh records p and reports whether it is new, counting it as accepted
// or as a duplicate.
func (s *search) fresh(p *puzzle.Puzzle) bool {
	id := collection.ID(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[id] {
		s.duplicates.Add(1)
		return false
	}
	s.seen[id] = true
	s.accepted.Add(1)
	return true
}

// rate fills in c's difficulty and reports whether c is now among the k
// hardest. The rules run first; only if they get stuck and c can still
// make it into the k hardest (harder looks at their free cells first) does
// the solver with search run, for the lookahead count and the steps. On an
// empty 14×14 pattern, the search costs four times as much as the
// completion, the rules alone a third.
func (s *search) rate(p *puzzle.Puzzle, c *Candidate) bool {
	res, err := s.rules(p)
	if err != nil {
		return false
	}
	if res.Status == solver.Stuck {
		c.RulesOut = freeCells(res.State)
		if !s.mayEnter(c) {
			return false
		}
		if res, err = s.solve(p); err != nil {
			return false
		}
		c.Stats = web.Stats{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses}
	}
	c.Status, c.Steps = res.Status.String(), len(res.Trace)
	return s.enter(c)
}

// mayEnter reports whether c, rated by the rules but not yet by the search,
// could be among the k hardest. Entering takes a candidate that is strictly
// harder than the last of best (see enter), and the search only adds
// lookahead where the rules get stuck.
func (s *search) mayEnter(c *Candidate) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.best) < s.k {
		return true
	}
	last := s.best[s.k-1].RulesOut
	return c.RulesOut > last || c.RulesOut == last && c.RulesOut > 0
}

// enter adds c to the k hardest if it is one of them, numbering it.
func (s *search) enter(c *Candidate) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.Entered = s.entered + 1
	if len(s.best) == s.k && !harder(c, s.best[s.k-1]) {
		return false
	}
	s.entered++
	i, _ := slices.BinarySearchFunc(s.best, c, func(a, b *Candidate) int {
		if harder(a, b) {
			return -1
		}
		return 1
	})
	s.best = slices.Insert(s.best, i, c)
	if len(s.best) > s.k {
		s.best = s.best[:s.k]
	}
	return true
}

// harder orders candidates hardest first: more free cells where the rules
// alone get stuck (the library collects puzzles the rules cannot solve),
// then more lookahead, then the one that entered the k hardest first, so an
// equally hard one never pushes out an earlier one. designer.js sorts the
// same way: the page keeps the first k of what it got, which then are the
// k hardest of the server, whatever order the candidates arrived in.
func harder(a, b *Candidate) bool {
	if a.RulesOut != b.RulesOut {
		return a.RulesOut > b.RulesOut
	}
	if a.Stats.Lookahead != b.Stats.Lookahead {
		return a.Stats.Lookahead > b.Stats.Lookahead
	}
	return a.Entered < b.Entered
}

func (s *search) counts() *designCounts {
	return &designCounts{Tried: s.tried.Load(), Accepted: s.accepted.Load(), Duplicates: s.duplicates.Load()}
}

// freeCells counts the free cells of st.
func freeCells(st *board.State) int {
	free := 0
	for r := range st.P.N {
		for c := range st.P.N {
			if st.At(r, c) == board.Free {
				free++
			}
		}
	}
	return free
}

var designName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`)

// saveDesign checks the chosen completion (valid, unique, contains the
// pattern, and with req.Strict no drawn region larger than drawn) and writes
// it to dir as nNN_name.txt, with its name for the library
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
	p, err := puzzle.New(req.Regions)
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
