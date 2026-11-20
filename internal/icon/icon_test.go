package icon

import (
	"bytes"
	"flag"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

var update = flag.Bool("update", false, "rewrite the icons in internal/web/assets/icons")

// dir holds the files the server embeds, relative to this package.
const dir = "../web/assets/icons"

// TestFiles checks that the icon files are what the code draws; with -update
// it writes them. Pixels may differ by a little: floating point may round
// differently on another CPU (fused multiply-add on arm64).
func TestFiles(t *testing.T) {
	for _, ic := range All {
		img := draw(ic)
		file := filepath.Join(dir, ic.File)
		if *update {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		f, err := os.Open(file)
		if err != nil {
			t.Errorf("%v; write it: go test ./internal/icon -update", err)
			continue
		}
		old, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", ic.File, err)
			continue
		}
		if old.Bounds() != img.Bounds() {
			t.Errorf("%s is %v, the code draws %v; rewrite it: go test ./internal/icon -update", ic.File, old.Bounds(), img.Bounds())
			continue
		}
		off := 0
		for y := range ic.Size {
			for x := range ic.Size {
				a := img.NRGBAAt(x, y)
				b := color.NRGBAModel.Convert(old.At(x, y)).(color.NRGBA)
				if a.A == 0 && b.A == 0 {
					continue
				}
				if diff(a.R, b.R) > 2 || diff(a.G, b.G) > 2 || diff(a.B, b.B) > 2 || diff(a.A, b.A) > 2 {
					off++
				}
			}
		}
		if off > 0 {
			t.Errorf("%s: %d pixels are not what the code draws; rewrite it: go test ./internal/icon -update", ic.File, off)
		}
	}
}

// The icon shows a real puzzle and its only solution.
func TestPuzzle(t *testing.T) {
	rs := make([][]int, len(regions))
	for r := range regions {
		rs[r] = regions[r][:]
	}
	p, err := puzzle.New(rs)
	if err != nil {
		t.Fatal(err)
	}
	sols := bruteforce.Solve(p, 2)
	if len(sols) != 1 || !slices.Equal(sols[0], stars[:]) {
		t.Fatalf("solutions %v, the icon shows %v", sols, stars)
	}
}

func diff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
