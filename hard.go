// Package starbattle holds data shared by the commands: the hard puzzles of
// testdata/hard, which the web server offers as a library and the tests use
// (a sample of them by default), the motifs drawn in the designer (designs/),
// the library's other part, and the catalog of star symbols in symbols/.
package starbattle

import (
	"embed"
	"os"

	"starbattle/internal/collection"
)

// Hard is testdata/hard, a collection (see package collection) plus the
// sample the tests run (sample.idx).
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
		index = collection.Index
	}
	return collection.ReadIndex(Hard, HardDir+"/"+index)
}

// Designs is designs/, the collection of motifs saved by the designer (see
// package collection).
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
