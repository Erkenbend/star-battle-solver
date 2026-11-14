package collection

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"starbattle/internal/puzzle"
)

// Unique (generate -n 5 -seed 3).
const p5 = "DBBBB\nDDBCC\nDDBAA\nEDDAA\nEAAAA\n"

func collFS(index string, files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{"c/" + Index: {Data: []byte(index)}}
	for name, text := range files {
		fsys["c/"+name] = &fstest.MapFile{Data: []byte(text)}
	}
	return fsys
}

func TestReadIndex(t *testing.T) {
	names, err := ReadIndex(collFS("# header\n\na.txt\n  b.txt  # comment\n", nil), "c/"+Index)
	if err != nil || strings.Join(names, ",") != "a.txt,b.txt" {
		t.Fatalf("got %q, %v", names, err)
	}
}

func TestLoad(t *testing.T) {
	entries, err := Load(collFS("a.txt\nb.txt\n", map[string]string{"a.txt": "# rating: 1\n" + p5, "b.txt": "AB\nAB\n"}), "c")
	if err != nil || len(entries) != 2 {
		t.Fatalf("got %d entries, %v", len(entries), err)
	}
	if e := entries[0]; e.File != "a.txt" || e.Puzzle.N != 5 || e.ID != ID(puzzle.MustParse(p5)) || Rating(e.Text) != 1 {
		t.Errorf("first entry %+v", e)
	}
}

func TestLoadErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{"no index", fstest.MapFS{}, "file does not exist"},
		{"listed twice", collFS("a.txt\na.txt\n", map[string]string{"a.txt": p5}), "a.txt doppelt"},
		{"missing file", collFS("a.txt\n", nil), "file does not exist"},
		{"unparsable", collFS("a.txt\n", map[string]string{"a.txt": "AB\n"}), "a.txt:"},
		{"same puzzle", collFS("a.txt\nb.txt\n", map[string]string{"a.txt": p5, "b.txt": strings.ReplaceAll(p5, "A", "F")}), "b.txt: dasselbe Puzzle wie a.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.fsys, "c")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

// TestID: the ID depends on the regions only, not on their letters.
func TestID(t *testing.T) {
	a := puzzle.MustParse(p5)
	b := puzzle.MustParse(strings.NewReplacer("A", "E", "E", "A", "C", "B", "B", "C").Replace(p5))
	if ID(a) != ID(b) {
		t.Errorf("renamed letters change the ID: %s vs %s", ID(a), ID(b))
	}
	other := puzzle.MustParse("CCAAA\nCCADA\nEDDDA\nEDEBB\nEEEBB\n")
	if ID(a) == ID(other) {
		t.Error("different regions, same ID")
	}
}

// TestWrite: creating files, appending to the index and finding puzzles by
// ID and names by stem, on disk as the tools do it.
func TestWrite(t *testing.T) {
	dir := t.TempDir()
	p := puzzle.MustParse(p5)
	if prev, err := Find(os.DirFS(dir), ".", p); err != nil || prev != "" {
		t.Fatalf("no index yet: %q, %v", prev, err)
	}
	file := FileName(5, "herz")
	if file != "n05_herz.txt" {
		t.Fatalf("FileName: %s", file)
	}
	if err := Create(dir, file, p5); err != nil {
		t.Fatal(err)
	}
	if err := Create(dir, file, "AB\nAB\n"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second Create: %v", err)
	}
	for _, f := range []string{file, "n04_x.txt"} {
		if err := AppendIndex(dir, f, "# header\n"); err != nil {
			t.Fatal(err)
		}
	}
	if index, _ := os.ReadFile(filepath.Join(dir, Index)); string(index) != "# header\nn05_herz.txt\nn04_x.txt\n" {
		t.Errorf("index:\n%s", index)
	}
	renamed := puzzle.MustParse(strings.ReplaceAll(p5, "A", "F"))
	if prev, err := Find(os.DirFS(dir), ".", renamed); err != nil || prev != file {
		t.Errorf("Find: %q, %v", prev, err) // n04_x.txt is missing: skipped
	}
	if got, err := WithStem(dir, "herz"); err != nil || got != file {
		t.Errorf("WithStem(herz): %q, %v", got, err)
	}
	if got, _ := WithStem(dir, "her"); got != "" {
		t.Errorf("WithStem(her): %q", got)
	}
	if !Exists(dir, file) || Exists(dir, "n05_d001.txt") {
		t.Error("Exists")
	}
}

func TestRating(t *testing.T) {
	for text, want := range map[string]int{
		"# generate ...\n# rating: 2 (4 lookahead steps, longest 5)\nAB\nBA\n": 2,
		"#rating:3\nAB\n":               0, // the key needs its space
		"# rating: 3\nAB\n":             3,
		"  # rating: 1 (rules alone)\n": 1,
		"# rating: 4\n":                 0,
		"# rating: 12\n":                0,
		"# rating:\n":                   0,
		"AB\nBA\n":                      0,
		"# name: Smiley\n":              0,
	} {
		if got := Rating([]byte(text)); got != want {
			t.Errorf("Rating(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestName(t *testing.T) {
	for _, tt := range []struct{ file, text, want string }{
		{"n08_sad_smiley.txt", "AAB\n", "Sad Smiley"},
		{"n10_herz-klein.txt", "", "Herz Klein"},
		{"x.txt", "", "X"},
		{"n08_a.txt", "# designer: …\n  # name:  Trauriger Smiley \nAB\n", "Trauriger Smiley"},
		{"n08_a.txt", "# name:\n", "A"},
	} {
		if got := Name(tt.file, []byte(tt.text)); got != tt.want {
			t.Errorf("Name(%q, %q) = %q, want %q", tt.file, tt.text, got, tt.want)
		}
	}
}

func TestSetHeader(t *testing.T) {
	for _, tt := range []struct{ text, want string }{
		{"# a\n# b\nAB\nBA\n", "# a\n# b\n# rating: 2 (x)\nAB\nBA\n"},      // added after the comments
		{"AB\nBA\n", "# rating: 2 (x)\nAB\nBA\n"},                          // no comments
		{"# a\n# rating: 3\n# b\nAB\n", "# a\n# rating: 2 (x)\n# b\nAB\n"}, // replaced in place
	} {
		if got := SetHeader(tt.text, RatingKey, "2 (x)"); got != tt.want {
			t.Errorf("SetHeader(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
