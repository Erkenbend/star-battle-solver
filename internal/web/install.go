package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strconv"
	"sync"
)

// Installing the site as a web app (decision 26): a manifest and icons, and
// the head tags that link them on every served page. No service worker, so
// nothing works offline; the app is the site in a window of its own.

// appHead goes into the <head> of every served page (the placeholder
// <!--{{APP}}--> of the templates); a page saved with `starbattle html` gets
// nothing, it has no server to fetch these from. The manifest is fetched with
// credentials: without them a site behind basic auth answers 401 and the
// page is not installable.
const appHead = `<link rel="manifest" href="/manifest.webmanifest" crossorigin="use-credentials">
<link rel="icon" href="/icon-32.png" sizes="32x32" type="image/png">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<meta name="theme-color" content="#f6f5f2" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#17171a" media="(prefers-color-scheme: dark)">`

// icon is one rendering of the app's icon.
type icon struct {
	size    int
	board   float64 // side of the board, as a part of the icon's side
	radius  float64 // corner radius of the background, as a part of the side; 0 square, full bleed
	purpose string  // in the manifest: "any" or "maskable"
}

// icons by URL. Maskable icons are cut to a circle or another shape by the
// launcher, so their board stays within the safe zone (a circle of 80 % of
// the side); iOS rounds the apple-touch-icon itself and fills transparent
// pixels with black, so it is square too.
var icons = map[string]icon{
	"/icon-32.png":           {size: 32, board: 1},
	"/icon-192.png":          {size: 192, board: 0.8, radius: 0.22, purpose: "any"},
	"/icon-512.png":          {size: 512, board: 0.8, radius: 0.22, purpose: "any"},
	"/icon-maskable-512.png": {size: 512, board: 0.56, purpose: "maskable"},
	"/apple-touch-icon.png":  {size: 180, board: 0.72},
}

// manifestIcons are the icons the manifest names, in this order.
var manifestIcons = []string{"/icon-192.png", "/icon-512.png", "/icon-maskable-512.png"}

// Manifest is the web app manifest.
type Manifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	Lang            string         `json:"lang"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []ManifestIcon `json:"icons"`
}

// ManifestIcon is an icon of the manifest.
type ManifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

func manifest() Manifest {
	m := Manifest{
		ID:              "/",
		Name:            "Star Battle",
		ShortName:       "Star Battle",
		Description:     "Star Battle lösen, allein oder zusammen, mit einem Lösungsweg, der jeden Schritt erklärt.",
		Lang:            "de",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: "#f6f5f2", // --bg of style.css, light theme
		ThemeColor:      "#f6f5f2",
	}
	for _, u := range manifestIcons {
		ic := icons[u]
		m.Icons = append(m.Icons, ManifestIcon{Src: u, Sizes: sizes(ic.size), Type: "image/png", Purpose: ic.purpose})
	}
	return m
}

func sizes(n int) string { return strconv.Itoa(n) + "x" + strconv.Itoa(n) }

// installRoutes serves the manifest and the icons. Both change only with a
// new binary; a day in the browser's cache is short enough for that.
func installRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "max-age=86400")
		json.NewEncoder(w).Encode(manifest())
	})
	for u, ic := range icons {
		data := sync.OnceValues(func() ([]byte, error) { return iconPNG(ic) })
		mux.HandleFunc("GET "+u, func(w http.ResponseWriter, r *http.Request) {
			b, err := data()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "max-age=86400")
			w.Write(b)
		})
	}
}

// The icon shows a 5×5 puzzle with its solution, in the colors the page
// gives a 5×5 board (Logic.regionHSL in logic.js, light theme). It is a real
// puzzle: the test checks that the stars are its only solution.
var (
	iconRegions = [5][5]int{
		{0, 0, 1, 1, 1},
		{0, 2, 2, 1, 1},
		{2, 2, 2, 3, 1},
		{4, 4, 3, 3, 1},
		{4, 4, 4, 4, 4},
	}
	iconStars = [5]int{1, 3, 0, 2, 4} // star column per row
	// regionHSL(5, r) for r = 0..4: palette entries 0, 4, 9, 2, 7.
	iconColors = [5]color.NRGBA{hsl(0, 72, 82), hsl(140, 40, 76), hsl(305, 40, 80), hsl(50, 85, 70), hsl(228, 65, 83)}

	iconBg       = color.NRGBA{0xf6, 0xf5, 0xf2, 0xff} // --bg
	iconThick    = color.NRGBA{0x1d, 0x1d, 0x1f, 0xff} // --thick
	iconThin     = color.NRGBA{0, 0, 0, 46}            // --thin, rgba(0, 0, 0, 0.18)
	iconStar     = color.NRGBA{0xf5, 0xb8, 0x00, 0xff} // --star
	iconStarEdge = color.NRGBA{0x5c, 0x3d, 0x00, 0xff} // --star-edge

	// The star symbol of symbols.json ("star"), in a box of 100.
	iconStarPath = [][2]float64{{50, 6}, {61.5, 37.2}, {94.7, 38.5}, {68.5, 59}, {77.6, 91}, {50, 72.5}, {22.4, 91}, {31.5, 59}, {5.3, 38.5}, {38.5, 37.2}}
)

// iconPNG renders ic, with 4×4 samples per pixel for smooth edges.
func iconPNG(ic icon) ([]byte, error) {
	const ss = 4
	img := image.NewNRGBA(image.Rect(0, 0, ic.size, ic.size))
	for py := 0; py < ic.size; py++ {
		for px := 0; px < ic.size; px++ {
			var r, g, b, a float64 // premultiplied sums
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(ic.size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(ic.size)
					c := iconAt(ic, x, y)
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
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// iconAt is the color of ic at (x, y), both from 0 to 1.
func iconAt(ic icon, x, y float64) color.NRGBA {
	if ic.radius > 0 && !inRoundedSquare(x, y, ic.radius) {
		return color.NRGBA{}
	}
	off := (1 - ic.board) / 2
	cell := ic.board / 5
	u, v := (x-off)/cell, (y-off)/cell // in cells
	if u < 0 || v < 0 || u >= 5 || v >= 5 {
		return iconBg
	}
	row, col := int(v), int(u)
	// Half widths of the lines, in cells. Lines are drawn inside the board:
	// the outer frame from its edge inwards. A small icon (the favicon)
	// keeps walls of about 2 px and loses the thin lines and the outline of
	// the stars, which would cover them.
	px := cell * float64(ic.size)
	small := px < 12
	thick, thin, edge, fill := math.Max(0.05, 0.9/px), 0.014, 3.5, 0.8
	if small {
		thin, edge, fill = 0, 0, 1
	}
	// The star, over the walls in a small icon. On the page the symbol fills
	// 92 % of the cell (.cell .sym) and its outline is 5 of 100; here the
	// walls are thicker, so both are a bit smaller.
	star := color.NRGBA{}
	if iconStars[row] == col {
		sx := (u - float64(col) - (1-fill)/2) / fill * 100
		sy := (v - float64(row) - (1-fill)/2) / fill * 100
		in, d := inPolygon(iconStarPath, sx, sy)
		switch {
		case d < edge:
			star = iconStarEdge
		case in:
			star = iconStar
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
			return iconRegions[rowOrCol][k-1] != iconRegions[rowOrCol][k]
		}
		return iconRegions[k-1][rowOrCol] != iconRegions[k][rowOrCol]
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
		return iconThick
	}
	if dv < thick && (wall(col, kv, false) || wall(int(u-thick), kv, false) || wall(int(u+thick), kv, false)) {
		return iconThick
	}
	if star.A > 0 {
		return star
	}
	c := iconColors[iconRegions[row][col]]
	if du < thin || dv < thin {
		return over(iconThin, c)
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
