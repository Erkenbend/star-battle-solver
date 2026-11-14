package sprite

// dog.svg: a dog face that tilts its head at you, pants with flapping ears,
// or barks.

const (
	dogInk    = "#3a2410"
	dogFur    = "#e3ad6e"
	dogEar    = "#8a5a32"
	dogPatch  = "#c98d4f"
	dogMuzzle = "#fff4e6"
	dogTongue = "#ff6f8e"
)

// dog is a frame of the dog, the zero value the resting pose.
type dog struct {
	tilt   float64 // turn of the head
	swing  float64 // turn of the ears
	tongue float64 // length of the tongue, 0 for 6
	bob    float64 // head lowered by
	q      float64 // opacity of a question mark
	bark   float64 // open mouth, 0 to 1; eyes squeezed shut at 1
	lines  float64 // opacity of the bark lines
}

func dogSheet() Sheet {
	return sheet("dog.svg", dog.draw, dog{}, []clip[dog]{
		// Tilts its head, a question mark appears.
		{"tilt", []dog{{tilt: 8, swing: 6}, {tilt: 16, swing: 10, tongue: 4, q: 0.5}, {tilt: 18, swing: 8, tongue: 4, q: 1},
			{tilt: 18, swing: 8, tongue: 4, q: 1}, {tilt: 18, swing: 8, tongue: 4, q: 1}, {tilt: 18, swing: 8, tongue: 4, q: 1},
			{tilt: 10, swing: 2, q: 0.5}, {}, {}}},
		// Pants happily, ears flapping.
		{"pant", []dog{{swing: -8, tongue: 12, bob: -2}, {swing: 8, tongue: 7, bob: 1},
			{swing: -8, tongue: 12, bob: -2}, {swing: 8, tongue: 7, bob: 1}, {swing: -4, tongue: 10, bob: -1}, {}}},
		// Barks twice: "wuff wuff".
		{"bark", []dog{{bob: -1, bark: 0.5, swing: 4}, {bob: -3, bark: 1, swing: 10, lines: 1}, {bob: -3, bark: 1, swing: 10, lines: 0.6},
			{bark: 0.3}, {}, {bob: -1, bark: 0.5, swing: 4}, {bob: -3, bark: 1, swing: 10, lines: 1},
			{bob: -3, bark: 1, swing: 10, lines: 0.6}, {bark: 0.3}, {}, {}}},
	})
}

func (d dog) draw() string {
	var s svg
	s.add(`<g transform="translate(0 %v) rotate(%v 50 60)">`, d.bob, d.tilt)
	const ear = `fill="` + dogEar + `" stroke="` + dogInk + `" stroke-width="4.5" stroke-linejoin="round"`
	s.add(`<path d="M27 26C10 24 2 44 7 64C10 76 22 76 25 62Z" %s transform="rotate(%v 26 28)"/>`, ear, d.swing)
	s.add(`<path d="M73 26C90 24 98 44 93 64C90 76 78 76 75 62Z" %s transform="rotate(%v 74 28)"/>`, ear, -d.swing)
	s.add(`<ellipse cx="50" cy="54" rx="31" ry="30" fill="%s" stroke="%s" stroke-width="4.5"/>`, dogFur, dogInk)
	s.add(`<ellipse cx="63" cy="45" rx="10" ry="9" fill="%s"/>`, dogPatch)
	s.add(`<ellipse cx="50" cy="68" rx="18" ry="13" fill="%s"/>`, dogMuzzle)
	for _, x := range []float64{38, 63} {
		if d.bark >= 1 {
			// Eyes squeezed shut while barking.
			turn := ""
			if x > 50 {
				turn = "rotate(180 " + num(x).String() + " 47)"
			}
			s.add(`<path d="M%v 44 %v 47 %v 50" fill="none" stroke="%s" stroke-width="3" `+
				`stroke-linecap="round" stroke-linejoin="round" transform="%s"/>`, x-5, x+4, x-5, dogInk, turn)
		} else {
			s.add(`<circle cx="%v" cy="46" r="5.2" fill="%s"/><circle cx="%v" cy="44.3" r="1.9" fill="#fff"/>`, x, dogInk, x+1.7)
		}
	}
	if b := d.bark; b != 0 {
		cy, rx, ry := 70+2*b, 7+3*b, 3+7*b
		s.add(`<ellipse cx="50" cy="%v" rx="%v" ry="%v" fill="#7a2230" stroke="%s" stroke-width="2.5"/>`, cy, rx, ry, dogInk)
		s.add(`<ellipse cx="50" cy="%v" rx="%v" ry="%v" fill="%s"/>`, cy+0.55*ry, 0.65*rx, 0.4*ry, dogTongue)
	} else {
		t := d.tongue
		if t == 0 {
			t = 6
		}
		s.add(`<path d="M45 70H55V%vA5 5 0 0 1 45 %vZ" fill="%s" stroke="%s" stroke-width="2.5" stroke-linejoin="round"/>`, 70+t, 70+t, dogTongue, dogInk)
		s.add(`<path d="M50 72V%v" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>`, 68+t)
		s.add(`<path d="M50 63V68M50 68Q44 74 39 69M50 68Q56 74 61 69" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round"/>`, dogInk)
	}
	s.add(`<ellipse cx="50" cy="59" rx="7.5" ry="5.5" fill="%s"/><ellipse cx="48" cy="57.4" rx="2.6" ry="1.5" fill="#fff" opacity="0.8"/>`, dogInk)
	s.add("</g>")
	if a := d.q; a != 0 {
		s.add(`<g opacity="%v" fill="none" stroke="%s" stroke-width="4" stroke-linecap="round">`+
			`<path d="M80 14C80 5 94 5 94 13C94 20 87 20 87 27"/></g>`+
			`<circle cx="87" cy="34" r="2.6" fill="%s" opacity="%v"/>`, a, dogInk, dogInk, a)
	}
	if a := d.lines; a != 0 {
		s.add(`<path d="M80 22 92 14M83 30 96 28M76 15 80 4M20 22 8 14M17 30 4 28M24 15 20 4" fill="none" stroke="%s" `+
			`stroke-width="3.5" stroke-linecap="round" opacity="%v"/>`, dogInk, a)
	}
	return s.String()
}
