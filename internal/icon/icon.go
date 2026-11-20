// Package icon draws the icons of the web app (decision 26): a 5×5 puzzle
// with its solution, in the colors, walls and star of the page. The server
// does not draw them; it embeds the files this package's test writes to
// internal/web/assets/icons (go test ./internal/icon -update) and checks
// against the code.
package icon

import (
	"image"
	"image/color"
	"math"
)

// Icon is one rendering of the icon.
type Icon struct {
	File    string  // in internal/web/assets/icons, also the URL path below /
	Size    int     // in pixels, square
	Board   float64 // side of the board, as a part of the icon's side
	Radius  float64 // corner radius of the background, as a part of the side; 0 square, full bleed
	Purpose string  // in the manifest: "any" or "maskable"; empty if not in the manifest
}

// All are the renderings. Maskable icons are cut to a circle or another
// shape by the launcher, so their board stays within the safe zone (a circle
// of 80 % of the side); iOS rounds the apple-touch-icon itself and fills
// transparent pixels with black, so it is square too. The favicon is the
// board alone.
var All = []Icon{
	{File: "icon-32.png", Size: 32, Board: 1},
	{File: "apple-touch-icon.png", Size: 180, Board: 0.72},
	{File: "icon-192.png", Size: 192, Board: 0.8, Radius: 0.22, Purpose: "any"},
	{File: "icon-512.png", Size: 512, Board: 0.8, Radius: 0.22, Purpose: "any"},
	{File: "icon-maskable-512.png", Size: 512, Board: 0.56, Purpose: "maskable"},
}

// The icon shows a 5×5 puzzle with its solution, in the colors the page
// gives a 5×5 board (Logic.regionHSL in logic.js, light theme). It is a real
// puzzle: the test checks that the stars are its only solution.
var (
	regions = [5][5]int{
		{0, 0, 1, 1, 1},
		{0, 2, 2, 1, 1},
		{2, 2, 2, 3, 1},
		{4, 4, 3, 3, 1},
		{4, 4, 4, 4, 4},
	}
	stars = [5]int{1, 3, 0, 2, 4} // star column per row
	// regionHSL(5, r) for r = 0..4: palette entries 0, 4, 9, 2, 7.
	colors = [5]color.NRGBA{hsl(0, 72, 82), hsl(140, 40, 76), hsl(305, 40, 80), hsl(50, 85, 70), hsl(228, 65, 83)}

	cBg       = color.NRGBA{0xf6, 0xf5, 0xf2, 0xff} // --bg
	cThick    = color.NRGBA{0x1d, 0x1d, 0x1f, 0xff} // --thick
	cThin     = color.NRGBA{0, 0, 0, 46}            // --thin, rgba(0, 0, 0, 0.18)
	cStar     = color.NRGBA{0xf5, 0xb8, 0x00, 0xff} // --star
	cStarEdge = color.NRGBA{0x5c, 0x3d, 0x00, 0xff} // --star-edge

	// The star symbol of symbols.json ("star"), in a box of 100.
	starPath = [][2]float64{{50, 6}, {61.5, 37.2}, {94.7, 38.5}, {68.5, 59}, {77.6, 91}, {50, 72.5}, {22.4, 91}, {31.5, 59}, {5.3, 38.5}, {38.5, 37.2}}
)

// draw renders ic, with 4×4 samples per pixel for smooth edges.
func draw(ic Icon) *image.NRGBA {
	const ss = 4
	img := image.NewNRGBA(image.Rect(0, 0, ic.Size, ic.Size))
	for py := 0; py < ic.Size; py++ {
		for px := 0; px < ic.Size; px++ {
			var r, g, b, a float64 // premultiplied sums
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(ic.Size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(ic.Size)
					c := at(ic, x, y)
					al := float64(c.A) / 255
					r += float64(c.R) * al
					g += float64(c.G) * al
					b += float64(c.B) * al
					a += al
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5), uint8(a/(ss*ss)*255 + 0.5)})
		}
	}
	return img
}

// at is the color of ic at (x, y), both from 0 to 1.
func at(ic Icon, x, y float64) color.NRGBA {
	if ic.Radius > 0 && !inRoundedSquare(x, y, ic.Radius) {
		return color.NRGBA{}
	}
	off := (1 - ic.Board) / 2
	cell := ic.Board / 5
	u, v := (x-off)/cell, (y-off)/cell // in cells
	if u < 0 || v < 0 || u >= 5 || v >= 5 {
		return cBg
	}
	row, col := int(v), int(u)
	// Half widths of the lines, in cells. Lines are drawn inside the board:
	// the outer frame from its edge inwards. A small icon (the favicon)
	// keeps walls of about 2 px and loses the thin lines and the outline of
	// the stars, which would cover them.
	px := cell * float64(ic.Size)
	small := px < 12
	thick, thin, edge, fill := math.Max(0.05, 0.9/px), 0.014, 3.5, 0.8
	if small {
		thin, edge, fill = 0, 0, 1
	}
	// The star, over the walls in a small icon. On the page the symbol fills
	// 92 % of the cell (.cell .sym) and its outline is 5 of 100; here the
	// walls are thicker, so both are a bit smaller.
	star := color.NRGBA{}
	if stars[row] == col {
		sx := (u - float64(col) - (1-fill)/2) / fill * 100
		sy := (v - float64(row) - (1-fill)/2) / fill * 100
		in, d := inPolygon(starPath, sx, sy)
		switch {
		case d < edge:
			star = cStarEdge
		case in:
			star = cStar
		}
	}
	if small && star.A > 0 {
		return star
	}
	wall := func(rowOrCol, k int, vertical bool) bool {
		if k <= 0 || k >= 5 {
			return true
		}
		if rowOrCol < 0 || rowOrCol >= 5 {
			return false
		}
		if vertical {
			return regions[rowOrCol][k-1] != regions[rowOrCol][k]
		}
		return regions[k-1][rowOrCol] != regions[k][rowOrCol]
	}
	// The distance to the nearest grid line k.
	near := func(p float64) (k int, d float64) {
		k = int(math.Round(p))
		d = math.Abs(p - float64(k))
		if k == 0 || k == 5 {
			d -= thick // the frame lies inside the board
		}
		return k, d
	}
	ku, du := near(u)
	kv, dv := near(v)
	// A wall at line k is also the wall of the row (or column) beside the
	// point's, so walls meet at the corners without a notch.
	if du < thick && (wall(row, ku, true) || wall(int(v-thick), ku, true) || wall(int(v+thick), ku, true)) {
		return cThick
	}
	if dv < thick && (wall(col, kv, false) || wall(int(u-thick), kv, false) || wall(int(u+thick), kv, false)) {
		return cThick
	}
	if star.A > 0 {
		return star
	}
	c := colors[regions[row][col]]
	if du < thin || dv < thin {
		return over(cThin, c)
	}
	return c
}

// inRoundedSquare tells whether (x, y) lies in the unit square with corners
// rounded by r.
func inRoundedSquare(x, y, r float64) bool {
	cx := math.Max(r, math.Min(1-r, x))
	cy := math.Max(r, math.Min(1-r, y))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}

// inPolygon tells whether (x, y) lies in the polygon (even-odd rule) and how
// far it is from its outline.
func inPolygon(poly [][2]float64, x, y float64) (bool, float64) {
	in, d := false, math.Inf(1)
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		if (a[1] > y) != (b[1] > y) && x < a[0]+(y-a[1])/(b[1]-a[1])*(b[0]-a[0]) {
			in = !in
		}
		dx, dy := b[0]-a[0], b[1]-a[1]
		t := math.Max(0, math.Min(1, ((x-a[0])*dx+(y-a[1])*dy)/(dx*dx+dy*dy)))
		d = math.Min(d, math.Hypot(x-a[0]-t*dx, y-a[1]-t*dy))
	}
	return in, d
}

// over blends the translucent color top over the opaque color bottom.
func over(top, bottom color.NRGBA) color.NRGBA {
	a := float64(top.A) / 255
	mix := func(t, b uint8) uint8 { return uint8(float64(t)*a + float64(b)*(1-a) + 0.5) }
	return color.NRGBA{mix(top.R, bottom.R), mix(top.G, bottom.G), mix(top.B, bottom.B), 0xff}
}

// hsl converts a CSS hsl() color (h in degrees, s and l in percent).
func hsl(h, s, l float64) color.NRGBA {
	s, l = s/100, l/100
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.NRGBA{to(r), to(g), to(b), 0xff}
}
