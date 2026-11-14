// Package sprite draws the animated sprite sheets of symbols/ (see
// symbols/README.md): frames of 100×100 side by side, first the resting pose
// (frame 0, the one that stays on the board), then the clips, of which the
// page plays one at random when a star is set.
//
// The sheets are checked in. After changing a drawing, rewrite them with
//
//	go test ./internal/sprite -update
//
// and fix "frames" and "clips" in symbols/symbols.json if the test asks for
// it.
package sprite

import (
	"fmt"
	"strconv"
	"strings"
)

// Sheet is a drawn sprite sheet.
type Sheet struct {
	File   string // file name in symbols/
	SVG    []byte
	Frames int    // number of frames, including the resting pose
	Clips  []Clip // as in symbols.json
}

// Clip is one animation of a sheet, as in symbols.json.
type Clip struct {
	Name   string `json:"name"`
	Start  int    `json:"start"`
	Frames int    `json:"frames"`
}

// Sheets draws all sheets.
func Sheets() []Sheet {
	return []Sheet{catSheet(), dogSheet(), hopSheet(), frogSheet(), chickSheet(), ghostSheet(), moonSheet()}
}

// clip is a named animation of frames F.
type clip[F any] struct {
	name   string
	frames []F
}

// sheet draws the resting pose rest and then the clips with draw.
func sheet[F any](file string, draw func(F) string, rest F, clips []clip[F]) Sheet {
	frames := []F{rest}
	s := Sheet{File: file}
	for _, c := range clips {
		s.Clips = append(s.Clips, Clip{Name: c.name, Start: len(frames), Frames: len(c.frames)})
		frames = append(frames, c.frames...)
	}
	s.Frames = len(frames)
	w := s.Frames * 100
	out := []string{fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d 100" width="%d" height="100">`, w, w)}
	for i, f := range frames {
		out = append(out, fmt.Sprintf(`<g transform="translate(%d 0)">%s</g>`, i*100, draw(f)))
	}
	out = append(out, "</svg>")
	s.SVG = []byte(strings.Join(out, "\n") + "\n")
	return s
}

// num prints a number as short as possible without losing precision.
type num float64

func (x num) String() string {
	if x == 0 {
		return "0" // also for -0
	}
	return strconv.FormatFloat(float64(x), 'f', -1, 64)
}

// svg collects the markup of a frame.
type svg struct{ strings.Builder }

// add appends fmt.Sprintf(format, a...), with float64 arguments for %v
// printed as num.
func (s *svg) add(format string, a ...any) {
	for i, v := range a {
		if x, ok := v.(float64); ok {
			a[i] = num(x)
		}
	}
	fmt.Fprintf(s, format, a...)
}

// heartPath is a heart's path data, centered at (x, y), k units wide.
func heartPath(x, y, k float64) string {
	h := k / 2
	var s svg
	s.add("M%v %vC%v %v %v %v %v %vC%v %v %v %v %v %vZ",
		x, y+0.7*h, x-1.3*h, y-0.1*h, x-h, y-1.1*h, x, y-0.45*h,
		x+h, y-1.1*h, x+1.3*h, y-0.1*h, x, y+0.7*h)
	return s.String()
}
