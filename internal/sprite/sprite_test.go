package sprite_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/sprite"
	"starbattle/internal/web"
)

var update = flag.Bool("update", false, "rewrite the sprite sheets in symbols/")

// TestSheetsUpToDate checks that the sheets in symbols/ are what the code
// draws, and that symbols.json has their frames and clips. With -update it
// rewrites the sheets first.
func TestSheetsUpToDate(t *testing.T) {
	syms, err := web.LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		t.Fatal(err)
	}
	for _, sheet := range sprite.Sheets() {
		file := filepath.Join("..", "..", "symbols", sheet.File)
		if *update {
			if err := os.WriteFile(file, sheet.SVG, 0o644); err != nil {
				t.Fatal(err)
			}
		} else if old, err := os.ReadFile(file); err != nil {
			t.Error(err)
		} else if !bytes.Equal(old, sheet.SVG) {
			t.Errorf("%s is not what the code draws; rewrite it: go test ./internal/sprite -update", sheet.File)
		}

		var sym *web.Symbol
		for i, s := range syms.List() {
			if s.Src == sheet.File {
				sym = &syms.List()[i]
			}
		}
		if sym == nil {
			t.Errorf("%s: not in %s", sheet.File, web.SymbolsFile)
			continue
		}
		var clips []sprite.Clip
		for _, c := range sym.Clips {
			clips = append(clips, sprite.Clip(c))
		}
		got, _ := json.Marshal(clips)
		want, _ := json.Marshal(sheet.Clips)
		if sym.Frames != sheet.Frames || !bytes.Equal(got, want) {
			t.Errorf("%s: symbol %q in %s needs\n\"frames\": %d, \"clips\": %s",
				sheet.File, sym.ID, web.SymbolsFile, sheet.Frames, strings.ReplaceAll(string(want), "},{", "}, {"))
		}
	}
}
