package web

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

// Unique (generate -n 5 -seed 3).
const lib5 = "DBBBB\nDDBCC\nDDBAA\nEDDAA\nEAAAA\n"

// Four 2×2 squares: two solutions.
const lib4Ambiguous = "AABB\nAABB\nCCDD\nCCDD\n"

func libFS(index string, files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{"lib/" + LibraryIndex: {Data: []byte(index)}}
	for name, text := range files {
		fsys["lib/"+name] = &fstest.MapFile{Data: []byte(text)}
	}
	return fsys
}

func TestLoadLibraryErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{"no index", fstest.MapFS{}, "file does not exist"},
		{"empty", libFS("# nothing\n", nil), "keine Puzzles"},
		{"listed twice", libFS("a.txt\na.txt\n", map[string]string{"a.txt": lib5}), "doppelt"},
		{"missing file", libFS("a.txt\n", nil), "file does not exist"},
		{"unparsable", libFS("a.txt\n", map[string]string{"a.txt": "AB\n"}), "a.txt:"},
		{"too small", libFS("a.txt\n", map[string]string{"a.txt": "A\n"}), "außerhalb"},
		{"same puzzle", libFS("a.txt\nb.txt\n", map[string]string{"a.txt": lib5, "b.txt": lib5}), "dasselbe Puzzle wie 5-1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadLibrary(tt.fsys, "lib", testSolve)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLibraryAmbiguous(t *testing.T) {
	lib, err := LoadLibrary(libFS("a.txt\n", map[string]string{"a.txt": lib4Ambiguous}), "lib", testSolve)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(testSolve, nil, nil, lib, nil)
	rec := get(h, "/bibliothek/weg?id="+lib.entries[0].id)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "2 Lösungen") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
