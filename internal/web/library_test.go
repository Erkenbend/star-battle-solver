package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/puzzle"
)

func testLibrary(t *testing.T) *Library {
	t.Helper()
	lib, err := LoadLibrary(starbattle.Hard, "testdata/hard", testSolve)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

// TestLibraryIndex: the index lists every puzzle of testdata/hard exactly once
// (LoadLibrary rejects duplicates), so a new file is not forgotten.
func TestLibraryIndex(t *testing.T) {
	lib := testLibrary(t)
	files, _ := filepath.Glob("../../testdata/hard/*.txt")
	if len(files) == 0 || lib.Len() != len(files) {
		t.Fatalf("index lists %d puzzles, testdata/hard has %d", lib.Len(), len(files))
	}
	for i, e := range lib.entries {
		if e.nr != i+1 || len(e.id) != 10 {
			t.Errorf("entry %d: nr %d, id %q", i, e.nr, e.id)
		}
	}
	// The first numbers are fixed: shared results name them.
	if e := lib.entries[0]; e.p.N != 5 {
		t.Errorf("Nr. 1 is %d×%d", e.p.N, e.p.N)
	}
}

// TestLibraryID: the ID depends on the regions only, not on their letters.
func TestLibraryID(t *testing.T) {
	text, err := os.ReadFile("../../testdata/hard/n05_s029.txt")
	if err != nil {
		t.Fatal(err)
	}
	a := puzzle.MustParse(string(text))
	b := puzzle.MustParse(strings.NewReplacer("A", "E", "E", "A", "C", "B", "B", "C").Replace(string(text)))
	if LibraryID(a) != LibraryID(b) {
		t.Errorf("renamed letters change the ID: %s vs %s", LibraryID(a), LibraryID(b))
	}
	other := puzzle.MustParse("CCAAA\nCCADA\nEDDDA\nEDEBB\nEEEBB\n")
	if LibraryID(a) == LibraryID(other) {
		t.Error("different regions, same ID")
	}
}

func TestLibraryServer(t *testing.T) {
	lib := testLibrary(t)
	h := Handler(testSolve, nil, nil, lib)

	rec := get(h, "/bibliothek")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="groups"`) {
		t.Fatalf("overview: %d", rec.Code)
	}
	for _, e := range lib.entries {
		if !strings.Contains(rec.Body.String(), `"id":"`+e.id+`"`) {
			t.Errorf("overview lacks Nr. %d", e.nr)
		}
	}

	// The first request comes before the solver: the page has no solution
	// and no trace yet, and starts the solver.
	e := lib.entries[0]
	rec = get(h, "/bibliothek?id="+e.id)
	if rec.Code != http.StatusOK {
		t.Fatalf("puzzle: %d %s", rec.Code, rec.Body)
	}
	d := pageData(t, rec.Body.String())
	if d.Library == nil || d.Library.ID != e.id || d.Library.Nr != 1 || len(d.Library.Entries) != lib.Len() {
		t.Fatalf("library info: %+v", d.Library)
	}
	if !d.Served || !d.Library.Pending || d.Solution != nil || d.Status != "pending" || len(d.Steps) != 0 || !strings.Contains(d.Title, "Nr. 1") {
		t.Errorf("first page: served %v, pending %v, solution %v, status %s, %d steps, title %q", d.Served, d.Library.Pending, d.Solution, d.Status, len(d.Steps), d.Title)
	}
	if d.Library.Entries[0].Regions != nil {
		t.Error("puzzle page carries the regions of all library puzzles")
	}

	// The trace waits for the solver.
	rec = get(h, "/bibliothek/weg?id="+e.id)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("trace: %d %s", rec.Code, rec.Body)
	}
	var tr Data
	if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Status != "solved" || tr.Solution == nil || len(tr.Steps) == 0 || tr.Library.Pending {
		t.Errorf("trace: status %s, solution %v, %d steps", tr.Status, tr.Solution, len(tr.Steps))
	}
	if rec := get(h, "/bibliothek/weg?id=nope"); rec.Code != http.StatusNotFound {
		t.Errorf("trace of unknown id: %d", rec.Code)
	}

	// Once solved, the page has it all.
	d = pageData(t, get(h, "/bibliothek?id="+e.id).Body.String())
	if d.Library.Pending || d.Status != "solved" || len(d.Steps) != len(tr.Steps) {
		t.Errorf("later page: pending %v, status %s, %d steps", d.Library.Pending, d.Status, len(d.Steps))
	}

	if rec := get(h, "/bibliothek?id=nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: %d", rec.Code)
	}
	if rec := get(Handler(testSolve, nil, nil, nil), "/bibliothek"); rec.Code != http.StatusNotFound {
		t.Errorf("without library: %d", rec.Code)
	}
}
