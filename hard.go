// Package starbattle holds data shared by the commands: the hard puzzles of
// testdata/hard, which the tests use and the web server offers as a library,
// and the catalog of star symbols in symbols/.
package starbattle

import "embed"

// Hard is testdata/hard: the puzzles (*.txt) and their order in the library
// (bibliothek.idx).
//
//go:embed testdata/hard
var Hard embed.FS

// Symbols is symbols/: the catalog of symbols the players can choose for
// their stars (symbols.json) and the image files it names. Further image
// formats need their own pattern here (a pattern without a match does not
// build).
//
//go:embed symbols/symbols.json symbols/*.svg
var Symbols embed.FS
