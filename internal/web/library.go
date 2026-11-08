package web

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// LibraryIndex is the file in a library directory that lists its puzzles in
// order: one file name per line, '#' starts a comment. The position is the
// puzzle's number, so new puzzles are only ever appended.
const LibraryIndex = "bibliothek.idx"

// Library is a fixed collection of (hard) puzzles to browse and solve without
// help. Each puzzle has a number for people and an ID derived from its regions
// for links and the browser's storage, so neither depends on the generator,
// the file name or the order.
type Library struct {
	solve   SolveFunc
	entries []*libEntry
	byID    map[string]*libEntry
}

type libEntry struct {
	nr   int
	id   string
	p    *puzzle.Puzzle
	once sync.Once
	sol  []int
	res  solver.Result
	err  error
}

// LoadLibrary reads the library in dir of fsys (see LibraryIndex).
func LoadLibrary(fsys fs.FS, dir string, solve SolveFunc) (*Library, error) {
	idx, err := fs.ReadFile(fsys, path.Join(dir, LibraryIndex))
	if err != nil {
		return nil, err
	}
	lib := &Library{solve: solve, byID: map[string]*libEntry{}}
	files := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(idx))
	for sc.Scan() {
		name, _, _ := strings.Cut(sc.Text(), "#")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
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
		e := &libEntry{nr: len(lib.entries) + 1, id: LibraryID(p), p: p}
		if prev := lib.byID[e.id]; prev != nil {
			return nil, fmt.Errorf("%s: dasselbe Puzzle wie Nr. %d", name, prev.nr)
		}
		lib.entries = append(lib.entries, e)
		lib.byID[e.id] = e
	}
	if len(lib.entries) == 0 {
		return nil, fmt.Errorf("%s: keine Puzzles", LibraryIndex)
	}
	return lib, sc.Err()
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

// Len is the number of puzzles.
func (lib *Library) Len() int { return len(lib.entries) }

// solved solves e once: its unique solution and the solver's trace.
func (lib *Library) solved(e *libEntry) (*libEntry, error) {
	e.once.Do(func() {
		sols := bruteforce.Solve(e.p, 2)
		if len(sols) != 1 {
			e.err = fmt.Errorf("Bibliothek Nr. %d hat %d Lösungen", e.nr, len(sols))
			return
		}
		e.sol = sols[0]
		e.res, e.err = lib.solve(e.p)
	})
	return e, e.err
}

// WarmUp solves all puzzles one after another in the background, so even the
// big ones open without waiting.
func (lib *Library) WarmUp() {
	go func() {
		start := time.Now()
		for _, e := range lib.entries {
			if _, err := lib.solved(e); err != nil {
				log.Printf("Bibliothek: %v", err)
			}
		}
		log.Printf("Bibliothek: %d Puzzles bereit (%.1f s)", len(lib.entries), time.Since(start).Seconds())
	}()
}

// LibraryInfo places a page's puzzle in the library.
type LibraryInfo struct {
	ID      string   `json:"id"`
	Nr      int      `json:"nr"`
	Entries []LibRef `json:"entries"` // all puzzles, for "next unsolved"
}

// LibRef is one puzzle of the library in the overview.
type LibRef struct {
	ID      string  `json:"id"`
	Nr      int     `json:"nr"`
	N       int     `json:"n"`
	Regions [][]int `json:"regions,omitempty"` // overview only
}

// LibraryPage is the data of the overview page.
type LibraryPage struct {
	Entries []LibRef `json:"entries"`
}

func (lib *Library) refs(regions bool) []LibRef {
	out := make([]LibRef, len(lib.entries))
	for i, e := range lib.entries {
		out[i] = LibRef{ID: e.id, Nr: e.nr, N: e.p.N}
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

// libraryRoutes adds the library to mux:
//
//	GET /bibliothek          overview
//	GET /bibliothek?id=...   a puzzle of the library
func libraryRoutes(mux *http.ServeMux, lib *Library) {
	mux.HandleFunc("GET /bibliothek", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			writeHTML(w, http.StatusOK, func(b io.Writer) error { return WriteLibrary(b, LibraryPage{Entries: lib.refs(true)}) })
			return
		}
		e := lib.byID[id]
		if e == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Nicht gefunden</title><p>Dieses Puzzle gibt es in der Bibliothek nicht. <a href="/bibliothek">Zur Bibliothek</a></p>`)
			return
		}
		if _, err := lib.solved(e); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		d := Build(fmt.Sprintf("Star Battle %d×%d · Bibliothek Nr. %d", e.p.N, e.p.N, e.nr), e.p, e.sol, e.res)
		d.Served = true
		d.Text = e.p.String()
		d.Library = &LibraryInfo{ID: e.id, Nr: e.nr, Entries: lib.refs(false)}
		writePage(w, d)
	})
}
