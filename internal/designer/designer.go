// Package designer is the tool behind `starbattle design`, for the project's
// developers and deliberately not part of serve: draw part of a puzzle, let
// the generator complete it in several ways at once, pick one and save it to
// a collection of the project (see package collection), usually designs/.
//
// The page is built like the others of internal/web (web.ServePage, with the
// shared style and logic.js), from its own assets/designer.html and
// assets/designer.js.
package designer

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
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

// MaxCandidates bounds the completions one request computes.
const MaxCandidates = 8

// designAttempts bounds the attempts of each completion, so a pattern the
// generator cannot complete gives an answer within seconds, not minutes.
const designAttempts = 300

// Page is the data of the designer page.
type Page struct {
	MinN       int    `json:"minN"`
	MaxN       int    `json:"maxN"`
	Dir        string `json:"dir"`
	Candidates int    `json:"candidates"` // default number of completions
	MaxCands   int    `json:"maxCandidates"`
}

// designRequest is the body of /api/check, /api/complete and /api/save.
type designRequest struct {
	Pattern    [][]int `json:"pattern"`              // region per cell, -1 = free
	Candidates int     `json:"candidates,omitempty"` // complete: how many
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
	Status   string    `json:"status"`    // solver result with full search
	Steps    int       `json:"steps"`     // steps of the solver's trace
	RulesOut int       `json:"rulesFree"` // free cells when the rules alone got stuck, 0 if they solve it
	Stats    web.Stats `json:"stats"`
	Millis   int64     `json:"millis"` // time to generate
}

// designEvent is one line of /api/complete's answer (NDJSON): the pattern's
// problem (then nothing else follows), a candidate as soon as it is found,
// or, last, done.
type designEvent struct {
	Problem   *patternProblem `json:"problem,omitempty"`
	Candidate *Candidate      `json:"candidate,omitempty"`
	Done      *designDone     `json:"done,omitempty"`
}

// designDone ends the answer of /api/complete.
type designDone struct {
	Found      int `json:"found"`
	Failed     int `json:"failed"`     // completions that found nothing (or were cancelled)
	Duplicates int `json:"duplicates"` // completions equal to an earlier one, not sent
}

// Handler serves the designer. Saved puzzles go to dir. /play
// shows a puzzle to try it, as in serve; with a symbol catalog (may be nil),
// its link to /einstellungen works too.
func Handler(dir string, solve web.SolveFunc, syms *web.Symbols) http.Handler {
	mux := http.NewServeMux()
	play := web.Handler(solve, nil, nil, nil, syms)
	mux.Handle("GET /play", play)
	if syms != nil {
		for _, p := range []string{"GET /einstellungen", "GET /symbole.json", "GET /symbole/{file}"} {
			mux.Handle(p, play)
		}
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		d := Page{MinN: web.MinN, MaxN: web.MaxN, Dir: dir, Candidates: 4, MaxCands: MaxCandidates}
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
		k := min(max(req.Candidates, 1), MaxCandidates)
		var d designDone
		for c := range completeDesign(req.Pattern, req.Strict, k, solve, r.Context().Done()) {
			switch {
			case c == nil:
				d.Failed++
			case c.duplicate:
				d.Duplicates++
			default:
				d.Found++
				send(designEvent{Candidate: &c.Candidate})
			}
		}
		send(designEvent{Done: &d})
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

// foundCandidate is a result of completeDesign.
type foundCandidate struct {
	Candidate
	duplicate bool // equal to a candidate sent earlier
}

// completeDesign computes k completions of pattern at the same time, each
// in its own goroutine with its own seed, and solves each to judge it. The
// generator's CPU limit (one core stays free) applies to all of them
// together. Each result comes out of the channel as soon as it is ready, so
// the page can show it while the others are still running; nil stands for a
// failed completion. The channel is closed after the last one.
func completeDesign(pattern [][]int, strict bool, k int, solve web.SolveFunc, done <-chan struct{}) <-chan *foundCandidate {
	n := len(pattern)
	out := make(chan *foundCandidate, k)
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range k {
		seed := uint64(rand.IntN(999_999) + 1) // short, as in serve; written into the saved file
		wg.Go(func() {
			start := time.Now()
			p, sol, err := generator.Complete(pattern, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{
				MinRegionSize: generator.DefaultMinRegionSize(n),
				MaxAttempts:   designAttempts,
				Workers:       1, // the candidates are the parallelism
				Strict:        strict,
				Done:          done,
			})
			if err != nil {
				out <- nil
				return
			}
			c := &foundCandidate{Candidate: Candidate{Seed: seed, Regions: p.Regions, Solution: sol, Millis: time.Since(start).Milliseconds()}}
			mu.Lock()
			id := collection.ID(p)
			c.duplicate = seen[id]
			seen[id] = true
			mu.Unlock()
			if !c.duplicate {
				if res, err := solve(p); err == nil {
					c.Status = res.Status.String()
					c.Steps = len(res.Trace)
					c.Stats = web.Stats{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses}
					c.RulesOut = freeWhenStuck(p, res.Trace)
				}
			}
			out <- c
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
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
