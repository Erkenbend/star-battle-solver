package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"starbattle"
	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// LibraryIndex is the file in a library directory that lists its puzzles in
// order: one file name per line, '#' starts a comment. A puzzle's number
// counts the puzzles of its size up to it, so new puzzles are only ever
// appended.
const LibraryIndex = "bibliothek.idx"

// Library is a fixed collection of puzzles to browse, one part of
// /bibliothek. There are two kinds:
//
//   - Hard: puzzles the simple rules fail on (testdata/hard), solved without
//     help. Their name is size and number, "10-3": the third 10×10 puzzle.
//   - Motifs: pictures drawn in the designer (designs/), played like solo
//     puzzles, with hints and checks. Their name is given in the file (see
//     MotifName).
//
// Each puzzle also has an ID derived from its regions for links and the
// browser's storage, so neither depends on the generator, the file name or
// the order.
type Library struct {
	kind    string // KindHard or KindMotifs
	solve   SolveFunc
	entries []*libEntry
	byID    map[string]*libEntry
}

// The kinds of library, as the page sees them (LibraryInfo.Kind).
const (
	KindHard   = "schwer"
	KindMotifs = "motive"
)

type libEntry struct {
	nr     int    // among the puzzles of size p.N, from 1
	title  string // a motif's name; empty for hard puzzles
	rating int    // a hard puzzle's difficulty, 1 to 3 stars (0: none in its file); motifs have none
	id     string
	p      *puzzle.Puzzle
	once   sync.Once
	ready  chan struct{} // closed once sol, res and err are set
	sol    []int
	res    solver.Result
	err    error
}

// LoadLibrary reads the hard puzzles in dir of fsys (see LibraryIndex).
func LoadLibrary(fsys fs.FS, dir string, solve SolveFunc) (*Library, error) {
	return loadLibrary(fsys, dir, KindHard, solve)
}

// LoadMotifs reads the motifs in dir of fsys (see LibraryIndex and MotifName).
func LoadMotifs(fsys fs.FS, dir string, solve SolveFunc) (*Library, error) {
	return loadLibrary(fsys, dir, KindMotifs, solve)
}

func loadLibrary(fsys fs.FS, dir, kind string, solve SolveFunc) (*Library, error) {
	names, err := starbattle.ReadIndex(fsys, path.Join(dir, LibraryIndex))
	if err != nil {
		return nil, err
	}
	lib := &Library{kind: kind, solve: solve, byID: map[string]*libEntry{}}
	files := map[string]bool{}
	titles := map[string]string{}
	perSize := map[int]int{}
	for _, name := range names {
		if files[name] {
			return nil, fmt.Errorf("%s: %s doppelt", LibraryIndex, name)
		}
		files[name] = true
		text, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, err
		}
		p, err := puzzle.Parse(bytes.NewReader(text))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if p.N < MinN || p.N > MaxN {
			return nil, fmt.Errorf("%s: Größe %d außerhalb von %d bis %d", name, p.N, MinN, MaxN)
		}
		perSize[p.N]++
		e := &libEntry{nr: perSize[p.N], id: LibraryID(p), p: p, ready: make(chan struct{})}
		if kind == KindHard {
			e.rating = starbattle.FileRating(text)
		}
		if kind == KindMotifs {
			e.title = MotifName(name, text)
			if prev, ok := titles[e.title]; ok {
				return nil, fmt.Errorf("%s: Name %q hat schon %s", name, e.title, prev)
			}
			titles[e.title] = name
		}
		if prev := lib.byID[e.id]; prev != nil {
			return nil, fmt.Errorf("%s: dasselbe Puzzle wie %s", name, prev.name())
		}
		lib.entries = append(lib.entries, e)
		lib.byID[e.id] = e
	}
	if len(lib.entries) == 0 {
		return nil, fmt.Errorf("%s: keine Puzzles", LibraryIndex)
	}
	return lib, nil
}

// LibraryID identifies a puzzle by its regions alone: the regions renumbered
// in order of first appearance (row by row), hashed. Renaming the letters or
// the file keeps the ID.
func LibraryID(p *puzzle.Puzzle) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:", p.N)
	renum := map[int]int{}
	for _, row := range p.Regions {
		for _, reg := range row {
			k, ok := renum[reg]
			if !ok {
				k = len(renum)
				renum[reg] = k
			}
			fmt.Fprintf(h, "%d,", k)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// MotifNameKey starts the line of a motif's file that gives its name:
// "# name: Trauriger Smiley". The puzzle parser skips it as a comment.
const MotifNameKey = "# name:"

// MotifName is the name of the motif in file (its base name) with contents
// text: the MotifNameKey line, or else the file name without the size prefix,
// '_' and '-' as spaces, each word capitalized ("n08_sad_smiley.txt": "Sad
// Smiley").
func MotifName(file string, text []byte) string {
	for _, line := range strings.Split(string(text), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), MotifNameKey); ok {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
	}
	return MotifNameFromFile(file)
}

// MotifNameFromFile makes a name from a motif's file name (see MotifName).
func MotifNameFromFile(file string) string {
	base := strings.TrimSuffix(path.Base(file), path.Ext(file))
	if len(base) > 4 && base[0] == 'n' && base[3] == '_' && isDigit(base[1]) && isDigit(base[2]) {
		base = base[4:]
	}
	words := strings.FieldsFunc(base, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// name is what people call e: a motif's name, or a hard puzzle's size and
// number, "10-3".
func (e *libEntry) name() string {
	if e.title != "" {
		return e.title
	}
	return fmt.Sprintf("%d-%d", e.p.N, e.nr)
}

// Len is the number of puzzles.
func (lib *Library) Len() int { return len(lib.entries) }

// start solves e in the background, once: its unique solution and the
// solver's trace. Nothing is solved ahead: a puzzle is solved when its page
// is first requested, and the page needs the result only once the puzzle is
// solved by hand, which takes longer than the solver even on a Raspberry Pi.
func (lib *Library) start(e *libEntry) {
	e.once.Do(func() {
		go func() {
			defer close(e.ready)
			t := time.Now()
			sols := bruteforce.Solve(e.p, 2)
			if len(sols) != 1 {
				e.err = fmt.Errorf("Bibliothek %s hat %d Lösungen", e.name(), len(sols))
				return
			}
			e.sol = sols[0]
			e.res, e.err = lib.solve(e.p)
			log.Printf("Bibliothek %s gelöst (%.1f s)", e.name(), time.Since(t).Seconds())
		}()
	})
}

// done reports whether e's result is there (started or not).
func (e *libEntry) done() bool {
	select {
	case <-e.ready:
		return true
	default:
		return false
	}
}

// data is the page data of e, with the solver's result if it is there.
func (lib *Library) data(e *libEntry) Data {
	title := fmt.Sprintf("Star Battle %d×%d · Bibliothek %s", e.p.N, e.p.N, e.name())
	if lib.kind == KindMotifs {
		title = fmt.Sprintf("Star Battle %d×%d · Motiv %s", e.p.N, e.p.N, e.name())
	}
	var d Data
	if e.done() && e.err == nil {
		d = Build(title, e.p, e.sol, e.res)
	} else {
		d = Build(title, e.p, nil, solver.Result{})
		d.Status, d.Steps = "pending", []Step{}
	}
	d.Served = true
	d.Text = e.p.String()
	d.Library = &LibraryInfo{Kind: lib.kind, ID: e.id, Name: e.name(), Rating: e.rating, Entries: lib.refs(false), Pending: d.Status == "pending"}
	return d
}

// LibraryInfo places a page's puzzle in the library.
type LibraryInfo struct {
	Kind    string   `json:"kind"` // KindHard or KindMotifs
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Rating  int      `json:"rating,omitempty"` // a hard puzzle's difficulty, 1 to 3 stars
	Entries []LibRef `json:"entries"`          // all puzzles, for "next unsolved"
	// The solver is still at work: solution and trace are missing, the page
	// gets them from /bibliothek/weg once it needs them.
	Pending bool `json:"pending,omitempty"`
}

// LibRef is one puzzle of the library in the overview.
type LibRef struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"` // "10-3", or a motif's name
	Nr      int     `json:"nr"`   // among the puzzles of size N, from 1
	N       int     `json:"n"`
	Rating  int     `json:"rating,omitempty"`  // a hard puzzle's difficulty, 1 to 3 stars
	Regions [][]int `json:"regions,omitempty"` // overview only
}

// LibraryPage is the data of the overview page: its parts, one per library.
type LibraryPage struct {
	Parts []LibraryPart `json:"parts"`
}

// LibraryPart is one library in the overview.
type LibraryPart struct {
	Kind    string   `json:"kind"`
	Entries []LibRef `json:"entries"`
}

func (lib *Library) refs(regions bool) []LibRef {
	out := make([]LibRef, len(lib.entries))
	for i, e := range lib.entries {
		out[i] = LibRef{ID: e.id, Name: e.name(), Nr: e.nr, N: e.p.N, Rating: e.rating}
		if regions {
			out[i].Regions = e.p.Regions
		}
	}
	return out
}

// WriteLibrary renders the overview page for d to w.
func WriteLibrary(w io.Writer, d LibraryPage) error {
	return render(w, libraryHTML, "Star Battle · Bibliothek", d, appJS, libraryJS)
}

// libraryRoutes adds the libraries to mux, all under /bibliothek; a puzzle
// is found by its ID in any of them:
//
//	GET /bibliothek              overview
//	GET /bibliothek?id=...       a puzzle of a library
//	GET /bibliothek/weg?id=...   its solution and trace as JSON (Data), waits for the solver
//
// A hard puzzle's page comes before the solver is done (it is needed only
// once the puzzle is solved by hand); a motif's page waits for it, as hints
// need the trace from the start.
func libraryRoutes(mux *http.ServeMux, libs []*Library) {
	find := func(id string) (*Library, *libEntry) {
		for _, lib := range libs {
			if e := lib.byID[id]; e != nil {
				return lib, e
			}
		}
		return nil, nil
	}
	// wait starts the solver on e and waits for it; false if the request is gone.
	wait := func(r *http.Request, lib *Library, e *libEntry) bool {
		lib.start(e)
		select {
		case <-e.ready:
			return true
		case <-r.Context().Done():
			return false
		}
	}
	mux.HandleFunc("GET /bibliothek/weg", func(w http.ResponseWriter, r *http.Request) {
		lib, e := find(r.URL.Query().Get("id"))
		if e == nil {
			http.Error(w, "unbekanntes Puzzle", http.StatusNotFound)
			return
		}
		if !wait(r, lib, e) {
			return
		}
		if e.err != nil {
			http.Error(w, e.err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(lib.data(e))
	})
	mux.HandleFunc("GET /bibliothek", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			var page LibraryPage
			for _, lib := range libs {
				page.Parts = append(page.Parts, LibraryPart{Kind: lib.kind, Entries: lib.refs(true)})
			}
			writeHTML(w, http.StatusOK, func(b io.Writer) error { return WriteLibrary(b, page) })
			return
		}
		lib, e := find(id)
		if e == nil {
			writeError(w, http.StatusNotFound, "Dieses Puzzle gibt es in der Bibliothek nicht.", Link{"/bibliothek", "Zur Bibliothek"})
			return
		}
		if lib.kind == KindMotifs {
			if !wait(r, lib, e) {
				return
			}
			if e.err != nil {
				writeError(w, http.StatusInternalServerError, e.err.Error(), Link{"/bibliothek", "Zur Bibliothek"})
				return
			}
		}
		d := lib.data(e) // before start: a finished result goes into the page
		lib.start(e)
		writePage(w, d)
	})
}
