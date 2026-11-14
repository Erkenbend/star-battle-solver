package web

import (
	"encoding/json"
	"fmt"
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
	perSize := map[int]int{}
	for i, e := range lib.entries {
		perSize[e.p.N]++
		if e.nr != perSize[e.p.N] || e.name() != fmt.Sprintf("%d-%d", e.p.N, e.nr) || len(e.id) != 10 {
			t.Errorf("entry %d: %s, id %q", i, e.name(), e.id)
		}
		// Every hard puzzle is rated (starbattle hard -rate).
		if e.rating < 1 || e.rating > 3 {
			t.Errorf("%s: rating %d", e.name(), e.rating)
		}
	}
	// The sample the tests run (sample.idx) is part of the library, once each.
	sample, err := starbattle.ReadIndex(starbattle.Hard, starbattle.HardDir+"/sample.idx")
	if err != nil || len(sample) == 0 {
		t.Fatalf("sample: %v", err)
	}
	inLib := map[string]bool{}
	names, _ := starbattle.ReadIndex(starbattle.Hard, starbattle.HardDir+"/"+LibraryIndex)
	for _, name := range names {
		inLib[name] = true
	}
	seen := map[string]bool{}
	for _, name := range sample {
		if !inLib[name] || seen[name] {
			t.Errorf("sample.idx: %s not in the library or listed twice", name)
		}
		seen[name] = true
	}
	// The names are fixed: shared results use them.
	if e := lib.entries[0]; e.name() != "5-1" {
		t.Errorf("first entry is %s", e.name())
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
	h := Handler(testSolve, nil, nil, []*Library{lib}, nil)

	rec := get(h, "/bibliothek")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="groups"`) {
		t.Fatalf("overview: %d", rec.Code)
	}
	for _, e := range lib.entries {
		if !strings.Contains(rec.Body.String(), `"id":"`+e.id+`"`) {
			t.Errorf("overview lacks %s", e.name())
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
	if d.Library == nil || d.Library.ID != e.id || d.Library.Name != "5-1" || len(d.Library.Entries) != lib.Len() {
		t.Fatalf("library info: %+v", d.Library)
	}
	if d.Library.Rating != e.rating || d.Library.Entries[0].Rating != e.rating {
		t.Errorf("rating %d / %d, want %d", d.Library.Rating, d.Library.Entries[0].Rating, e.rating)
	}
	if !d.Served || !d.Library.Pending || d.Solution != nil || d.Status != "pending" || len(d.Steps) != 0 || !strings.Contains(d.Title, "Bibliothek 5-1") {
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

	if rec := get(h, "/bibliothek?id=nope"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `href="/bibliothek">Zur Bibliothek`) {
		t.Errorf("unknown id: %d", rec.Code)
	}
	if rec := get(Handler(testSolve, nil, nil, nil, nil), "/bibliothek"); rec.Code != http.StatusNotFound {
		t.Errorf("without library: %d", rec.Code)
	}
}

func testMotifs(t *testing.T) *Library {
	t.Helper()
	lib, err := LoadMotifs(starbattle.Designs, starbattle.DesignsDir, testSolve)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

// TestMotifsIndex: the index lists every file of designs/ once, each has its
// name in the file, and no motif is also a hard puzzle (IDs are looked up in
// both).
func TestMotifsIndex(t *testing.T) {
	motifs := testMotifs(t)
	for _, e := range motifs.entries { // only hard puzzles are rated
		if e.rating != 0 {
			t.Errorf("motif %s has rating %d", e.name(), e.rating)
		}
	}
	files, _ := filepath.Glob("../../designs/*.txt")
	if len(files) == 0 || motifs.Len() != len(files) {
		t.Fatalf("index lists %d motifs, designs has %d", motifs.Len(), len(files))
	}
	for _, f := range files {
		text, _ := os.ReadFile(f)
		if !strings.HasPrefix(string(text), MotifNameKey+" ") {
			t.Errorf("%s: no %q line at the top", filepath.Base(f), MotifNameKey)
		}
	}
	hard := testLibrary(t)
	for _, e := range motifs.entries {
		if hard.byID[e.id] != nil {
			t.Errorf("motif %s is also hard puzzle %s", e.name(), hard.byID[e.id].name())
		}
	}
}

func TestMotifName(t *testing.T) {
	for _, tt := range []struct{ file, text, want string }{
		{"n08_sad_smiley.txt", "AAB\n", "Sad Smiley"},
		{"n10_herz-klein.txt", "", "Herz Klein"},
		{"x.txt", "", "X"},
		{"n08_a.txt", "# designer: …\n  # name:  Trauriger Smiley \nAB\n", "Trauriger Smiley"},
		{"n08_a.txt", "# name:\n", "A"},
	} {
		if got := MotifName(tt.file, []byte(tt.text)); got != tt.want {
			t.Errorf("MotifName(%q, %q) = %q, want %q", tt.file, tt.text, got, tt.want)
		}
	}
}

func TestMotifsServer(t *testing.T) {
	motifs, hard := testMotifs(t), testLibrary(t)
	h := Handler(testSolve, nil, nil, []*Library{motifs, hard}, nil)

	rec := get(h, "/bibliothek")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"kind":"motive"`) || !strings.Contains(body, `"kind":"schwer"`) {
		t.Fatalf("overview: %d", rec.Code)
	}
	for _, e := range append(motifs.entries, hard.entries...) {
		if !strings.Contains(body, `"id":"`+e.id+`"`) {
			t.Errorf("overview lacks %s", e.name())
		}
	}

	// A motif's page waits for the solver: hints need the trace.
	e := motifs.entries[0]
	d := pageData(t, get(h, "/bibliothek?id="+e.id).Body.String())
	if d.Library == nil || d.Library.Kind != KindMotifs || d.Library.Name != "Simple" || len(d.Library.Entries) != motifs.Len() {
		t.Fatalf("library info: %+v", d.Library)
	}
	if d.Library.Pending || d.Status != "solved" || d.Solution == nil || len(d.Steps) == 0 || !strings.Contains(d.Title, "Motiv Simple") {
		t.Errorf("motif page: pending %v, status %s, %d steps, title %q", d.Library.Pending, d.Status, len(d.Steps), d.Title)
	}
	// Hard puzzles stay where they were, in their own list.
	d = pageData(t, get(h, "/bibliothek?id="+hard.entries[0].id).Body.String())
	if d.Library == nil || d.Library.Kind != KindHard || d.Library.Name != "5-1" || len(d.Library.Entries) != hard.Len() {
		t.Fatalf("hard library info: %+v", d.Library)
	}
}

func TestLoadMotifsDuplicateName(t *testing.T) {
	fsys := libFS("n05_a.txt\nb.txt\n", map[string]string{"n05_a.txt": lib5, "b.txt": "# name: A\n" + lib4Ambiguous})
	if _, err := LoadMotifs(fsys, "lib", testSolve); err == nil || !strings.Contains(err.Error(), `Name "A" hat schon n05_a.txt`) {
		t.Fatalf("got %v", err)
	}
}
