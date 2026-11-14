// Package starbattle holds data shared by the commands: the hard puzzles of
// testdata/hard, which the web server offers as a library and the tests use
// (a sample of them by default), the motifs drawn in the designer (designs/),
// the library's other part, and the catalog of star symbols in symbols/.
package starbattle

import (
	"bufio"
	"bytes"
	"embed"
	"io/fs"
	"os"
	"strings"
)

// Hard is testdata/hard: the puzzles (*.txt), their order in the library
// (bibliothek.idx) and the sample the tests run (sample.idx).
//
//go:embed testdata/hard
var Hard embed.FS

// HardDir is the directory of Hard, also relative to the module root.
const HardDir = "testdata/hard"

// AllHardEnv set to 1 makes HardTestSet return every puzzle of the library
// instead of the sample: STARBATTLE_ALL_HARD=1 go test ./...
const AllHardEnv = "STARBATTLE_ALL_HARD"

// HardTestSet names the puzzles of testdata/hard the tests run: the sample in
// sample.idx, chosen to cover all sizes and rules at a fraction of the time,
// or every puzzle in bibliothek.idx if AllHardEnv is set to 1.
func HardTestSet() ([]string, error) {
	index := "sample.idx"
	if os.Getenv(AllHardEnv) == "1" {
		index = "bibliothek.idx"
	}
	return ReadIndex(Hard, HardDir+"/"+index)
}

// ReadIndex reads an index file of fsys: one file name per line, '#' starts
// a comment, empty lines are skipped.
func ReadIndex(fsys fs.FS, name string) ([]string, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, sc.Err()
}

// Designs is designs/: the motifs saved by the designer (*.txt, each with
// its name in a "# name:" line) and their order in the library
// (bibliothek.idx).
//
//go:embed designs
var Designs embed.FS

// DesignsDir is the directory of Designs, also relative to the module root.
const DesignsDir = "designs"

// Symbols is symbols/: the catalog of symbols the players can choose for
// their stars (symbols.json) and the image files it names. Further image
// formats need their own pattern here (a pattern without a match does not
// build).
//
//go:embed symbols/symbols.json symbols/*.svg
var Symbols embed.FS
