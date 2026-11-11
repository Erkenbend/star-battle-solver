package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// The designer (starbattle design) is a tool for the project's developers,
// not part of serve: draw part of a puzzle, let the generator complete it in
// several ways at once, pick one and save it to a directory of the project.

// MaxDesignCandidates bounds the completions one request computes.
const MaxDesignCandidates = 8

// designAttempts bounds the attempts of each completion, so a pattern the
// generator cannot complete gives an answer within seconds, not minutes.
const designAttempts = 300

// DesignerPage is the data of the designer page.
type DesignerPage struct {
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
}

// patternProblem is the answer to /api/check, and part of /api/complete's
// answer if the pattern is not valid.
type patternProblem struct {
	Message string   `json:"message"`
	Cells   [][2]int `json:"cells,omitempty"`
}

// DesignCandidate is one completion of a pattern.
type DesignCandidate struct {
	Seed     uint64  `json:"seed"`
	Regions  [][]int `json:"regions"`
	Solution []int   `json:"solution"`
	Status   string  `json:"status"`    // solver result with full search
	Steps    int     `json:"steps"`     // steps of the solver's trace
	RulesOut int     `json:"rulesFree"` // free cells when the rules alone got stuck, 0 if they solve it
	Stats    Stats   `json:"stats"`
	Millis   int64   `json:"millis"` // time to generate
}

type completeResponse struct {
	Problem    *patternProblem   `json:"problem,omitempty"`
	Candidates []DesignCandidate `json:"candidates"`
	Failed     int               `json:"failed"` // completions that found nothing
}

// DesignerHandler serves the designer. Saved puzzles go to dir. /play
// shows a puzzle to try it, as in serve.
func DesignerHandler(dir string, solve SolveFunc) http.Handler {
	mux := http.NewServeMux()
	play := Handler(solve, nil, nil, nil)
	mux.Handle("GET /play", play)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		d := DesignerPage{MinN: MinN, MaxN: MaxN, Dir: dir, Candidates: 4, MaxCands: MaxDesignCandidates}
		writeHTML(w, http.StatusOK, func(b io.Writer) error {
			return render(b, designerHTML, "Star Battle · Designer", d, appJS, designerJS)
		})
	})
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
		req, ok := readDesignRequest(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, checkPattern(req.Pattern))
	})
	mux.HandleFunc("POST /api/complete", func(w http.ResponseWriter, r *http.Request) {
		req, ok := readDesignRequest(w, r)
		if !ok {
			return
		}
		if prob := checkPattern(req.Pattern); prob != nil {
			writeJSON(w, http.StatusOK, completeResponse{Problem: prob, Candidates: []DesignCandidate{}})
			return
		}
		k := min(max(req.Candidates, 1), MaxDesignCandidates)
		writeJSON(w, http.StatusOK, completeDesign(req.Pattern, k, solve, r.Context().Done()))
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
	if n < MinN || n > MaxN {
		http.Error(w, fmt.Sprintf("Die Größe muss zwischen %d und %d liegen.", MinN, MaxN), http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// checkPattern runs generator.CheckPattern and explains its finding in German,
// or returns nil if the pattern is fine.
func checkPattern(pattern [][]int) *patternProblem {
	n := len(pattern)
	minSize := generator.DefaultMinRegionSize(n)
	err := generator.CheckPattern(pattern, minSize)
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
		p.Message = color + ": Der markierte Teil lässt sich über freie Felder nicht mit dem Rest der Farbe verbinden. Jede Farbe muss am Ende eine zusammenhängende Fläche sein."
	case generator.TooSmall:
		p.Message = fmt.Sprintf("%s ist ringsum eingeschlossen und hat nur %d Feld(er). Bei %d×%d braucht jede Farbe mindestens %d Felder.", color, len(pe.Cells), n, n, minSize)
	case generator.Ambiguous:
		p.Message = "Die Sterne lassen sich allein auf gezeichneten Feldern auf mehrere Arten verteilen (markiert: wo sich zwei solche Verteilungen unterscheiden). Gezeichnete Felder ändert der Generator nie, also hätte jedes fertige Puzzle mehrere Lösungen. Einige Felder wieder freigeben hilft."
	default:
		p.Message = "Zu diesem Muster passt keine Verteilung der Sterne (je ein Stern pro Zeile, Spalte und Farbe, ohne Berührung). Das passiert z. B., wenn eine Farbe zwei ganze Zeilen füllt oder zwei eingeschlossene Farben nur in derselben Zeile Platz für ihren Stern haben."
	}
	return p
}

// completeDesign computes k completions of pattern at the same time, each
// in its own goroutine with its own seed, and solves each to judge it. The
// generator's CPU limit (one core stays free) applies to all of them
// together. Completions that fail are counted; identical ones are dropped.
func completeDesign(pattern [][]int, k int, solve SolveFunc, done <-chan struct{}) completeResponse {
	n := len(pattern)
	results := make([]*DesignCandidate, k)
	var wg sync.WaitGroup
	for i := range k {
		seed := newSeed()
		wg.Go(func() {
			start := time.Now()
			p, sol, err := generator.Complete(pattern, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{
				MinRegionSize: generator.DefaultMinRegionSize(n),
				MaxAttempts:   designAttempts,
				Workers:       1, // the candidates are the parallelism
				Done:          done,
			})
			if err != nil {
				return
			}
			c := &DesignCandidate{Seed: seed, Regions: p.Regions, Solution: sol, Millis: time.Since(start).Milliseconds()}
			if res, err := solve(p); err == nil {
				c.Status = res.Status.String()
				c.Steps = len(res.Trace)
				c.Stats = Stats{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses}
				c.RulesOut = freeWhenStuck(p, res.Trace)
			}
			results[i] = c
		})
	}
	wg.Wait()
	out := completeResponse{Candidates: []DesignCandidate{}}
	seen := map[string]bool{}
	for _, c := range results {
		if c == nil {
			out.Failed++
			continue
		}
		id := LibraryID(&puzzle.Puzzle{N: n, Regions: c.Regions})
		if seen[id] {
			continue
		}
		seen[id] = true
		out.Candidates = append(out.Candidates, *c)
	}
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
// pattern) and writes it to dir as nNN_name.txt, with the seed and the
// pattern in comments. Without a name, the next free nNN_dNNN is used. An
// existing file is never overwritten. Returns the file name.
func saveDesign(dir string, req designRequest) (string, error) {
	n := len(req.Pattern)
	p, err := puzzle.New(req.Regions)
	if err != nil {
		return "", fmt.Errorf("Puzzle ungültig: %v", err)
	}
	if p.N != n {
		return "", errors.New("Puzzle und Muster sind verschieden groß.")
	}
	for r, row := range req.Pattern {
		if len(row) != n {
			return "", errors.New("Muster ist nicht quadratisch.")
		}
		for c, reg := range row {
			if reg != generator.Free && reg != p.Regions[r][c] {
				return "", fmt.Errorf("Feld (%d,%d) passt nicht zum Muster.", r, c)
			}
		}
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
	prefix := fmt.Sprintf("n%02d_", n)
	if name == "" {
		for i := 1; ; i++ {
			name = fmt.Sprintf("d%03d", i)
			if _, err := os.Stat(filepath.Join(dir, prefix+name+".txt")); errors.Is(err, os.ErrNotExist) {
				break
			}
		}
	}
	file := prefix + name + ".txt"

	var b strings.Builder
	fmt.Fprintf(&b, "# designer: generator.Complete(pattern, rand.NewPCG(%d, %d)), %s\n", req.Seed, n, time.Now().Format("2006-01-02"))
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

	// O_EXCL: two saves with the same name must not overwrite each other.
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%s gibt es schon.", file)
	}
	if err != nil {
		return "", err
	}
	_, err = io.WriteString(f, b.String())
	return file, errors.Join(err, f.Close())
}
