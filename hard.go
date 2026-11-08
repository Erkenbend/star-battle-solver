// Package starbattle holds data shared by the commands: the hard puzzles of
// testdata/hard, which the tests use and the web server offers as a library.
package starbattle

import "embed"

// Hard is testdata/hard: the puzzles (*.txt) and their order in the library
// (bibliothek.idx).
//
//go:embed testdata/hard
var Hard embed.FS
