package sprite

// cat.svg: a cat face that sticks its tongue out, twitches an ear and grins,
// yawns, or falls in love.

const (
	catInk    = "#4a2a12"
	catFur    = "#f4a340"
	catStripe = "#d17a26"
	catCheek  = "#fff3e0"
	catPink   = "#f28aa0"
	catTongue = "#ff6f8e"
	catRed    = "#e8395a"
)

// cat is a frame of the cat, the zero value the resting pose.
type cat struct {
	ear    float64 // turn of the right ear
	eyes   string  // "" (open), "shut", "happy" or "heart"
	tongue float64 // how far the tongue sticks out
	tilt   float64 // turn of the head
	gape   float64 // yawn, 0 to 1
	lift   float64 // head raised by
	ears   float64 // both ears laid flat by
	heart  float64 // a heart floating up, 0 to 1
}

func catSheet() Sheet {
	return sheet("cat.svg", cat.draw, cat{}, []clip[cat]{
		// Sticks the tongue out.
		{"mlem", []cat{{tongue: 5}, {eyes: "shut", tongue: 10, tilt: -4}, {eyes: "shut", tongue: 10, tilt: -4},
			{eyes: "shut", tongue: 10, tilt: -4}, {tongue: 5}, {}, {}, {}}},
		// Twitches an ear, blinks and grins.
		{"ears", []cat{{ear: -26}, {}, {ear: -26}, {}, {eyes: "shut"}, {}, {eyes: "happy", tilt: 6},
			{eyes: "happy", tilt: 6, ear: -12}, {eyes: "happy", tilt: 6}, {}}},
		// Yawns with flat ears.
		{"yawn", []cat{{eyes: "shut"}, {eyes: "shut", gape: 0.4, lift: 0.5, ears: 6}, {eyes: "shut", gape: 0.8, lift: 1, ears: 12},
			{eyes: "shut", gape: 1, lift: 1, ears: 14}, {eyes: "shut", gape: 1, lift: 1, ears: 14},
			{eyes: "shut", gape: 1, lift: 1, ears: 14}, {eyes: "shut", gape: 0.6, lift: 0.5, ears: 8},
			{eyes: "shut", ears: 2}, {eyes: "shut"}, {}, {}, {}}},
		// Falls in love: heart eyes, a heart floats up.
		{"love", []cat{{eyes: "heart", tilt: -5}, {eyes: "heart", tilt: -5, heart: 0.15}, {eyes: "heart", tilt: -5, heart: 0.35},
			{eyes: "heart", tilt: -5, heart: 0.55}, {eyes: "heart", tilt: -5, heart: 0.75},
			{eyes: "heart", tilt: -5, heart: 0.95}, {eyes: "happy"}, {}}},
	})
}

func (c cat) draw() string {
	var s svg
	s.add(`<g transform="translate(0 %v) rotate(%v 50 62)">`, -c.lift, c.tilt)
	// The ears: the right one turned by ear, both laid flat by ears.
	catEar(&s, "14,50 20,10 46,32", "21,40 24,20 37,33", -c.ears, "28 38")
	catEar(&s, "86,50 80,10 54,32", "79,40 76,20 63,33", c.ear+c.ears, "72 38")
	s.add(`<ellipse cx="50" cy="60" rx="39" ry="31" fill="%s" stroke="%s" stroke-width="4.5"/>`, catFur, catInk)
	// Tabby stripes on the forehead.
	s.add(`<path d="M50 30V40M41 31 43 39M59 31 57 39" stroke="%s" stroke-width="3.5" stroke-linecap="round"/>`, catStripe)
	s.add(`<ellipse cx="25" cy="69" rx="6" ry="3.5" fill="%s" opacity="0.55"/>`+
		`<ellipse cx="75" cy="69" rx="6" ry="3.5" fill="%s" opacity="0.55"/>`, catPink, catPink)
	s.add(`<ellipse cx="50" cy="73" rx="15" ry="10" fill="%s"/>`, catCheek)
	for _, x := range []float64{36, 64} {
		switch c.eyes {
		case "":
			s.add(`<ellipse cx="%v" cy="55" rx="5" ry="7" fill="%s"/><circle cx="%v" cy="52.4" r="2" fill="#fff"/>`, x, catInk, x+1.6)
		case "heart":
			s.add(`<path d="%s" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`, heartPath(x, 55, 15), catRed, catInk)
		case "shut":
			s.add(`<path d="M%v 56 Q%v 60 %v 56" fill="none" stroke="%s" stroke-width="3.5" stroke-linecap="round"/>`, x-6, x, x+6, catInk)
		default: // happy
			s.add(`<path d="M%v 57 Q%v 49 %v 57" fill="none" stroke="%s" stroke-width="3.5" stroke-linecap="round"/>`, x-6, x, x+6, catInk)
		}
	}
	if g := c.gape; g != 0 {
		// Wide open mouth with fangs and the tongue at the bottom.
		cy, rx, ry := 73+4*g, 6+4*g, 2+9*g
		s.add(`<ellipse cx="50" cy="%v" rx="%v" ry="%v" fill="#7a2230" stroke="%s" stroke-width="2.5"/>`, cy, rx, ry, catInk)
		s.add(`<ellipse cx="50" cy="%v" rx="%v" ry="%v" fill="%s"/>`, cy+0.55*ry, 0.7*rx, 0.4*ry, catTongue)
		s.add(`<path d="M%v %vl2 %v 2 %vZM%v %vl-2 %v-2 %vZ" fill="#fff"/>`,
			50-0.6*rx, cy-0.85*ry, 2+3*g, -2-3*g, 50+0.6*rx, cy-0.85*ry, 2+3*g, -2-3*g)
	} else if t := c.tongue; t != 0 {
		s.add(`<path d="M45 72H55V%vA5 5 0 0 1 45 %vZ" fill="%s" stroke="%s" stroke-width="2.5" stroke-linejoin="round"/>`, 72+t, 72+t, catTongue, catInk)
		s.add(`<path d="M50 74V%v" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>`, 70+t)
	}
	s.add(`<path d="M46 65H54L50 70Z" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`, catPink, catInk)
	if c.gape == 0 {
		s.add(`<path d="M50 70Q50 76 44 75M50 70Q50 76 56 75" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round"/>`, catInk)
	}
	s.add(`<path d="M22 66 6 62M22 71 6 73M78 66 94 62M78 71 94 73" stroke="%s" stroke-width="2" stroke-linecap="round"/>`, catInk)
	s.add("</g>")
	if h := c.heart; h != 0 {
		a := min(1, 2.5*(1-h))
		s.add(`<path d="%s" fill="%s" stroke="%s" stroke-width="2" stroke-linejoin="round" opacity="%.2f"/>`,
			heartPath(84+4*h, 30-22*h, 12+6*h), catRed, catInk, a)
	}
	return s.String()
}

// catEar draws an ear of the polygon points with the inner polygon inner,
// turned by rot about pivot.
func catEar(s *svg, points, inner string, rot float64, pivot string) {
	s.add("<g")
	if rot != 0 {
		s.add(` transform="rotate(%v %s)"`, rot, pivot)
	}
	s.add(`><polygon points="%s" fill="%s" stroke="%s" stroke-width="4.5" stroke-linejoin="round"/>`+
		`<polygon points="%s" fill="%s"/></g>`, points, catFur, catInk, inner, catPink)
}
