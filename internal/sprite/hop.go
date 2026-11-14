package sprite

// hop.svg: a smiling star that hops with a wink, jumps and spins, or wobbles
// happily.

const (
	hopInk    = "#5c3d00"
	hopStar   = "50,6 61.5,37.2 94.7,38.5 68.5,59 77.6,91 50,72.5 22.4,91 31.5,59 5.3,38.5 38.5,37.2"
	hopBase   = 0.86 // star size at rest, so jumps stay inside the box
	hopGround = 94
)

// hop is a frame of the star, the zero value the resting pose.
type hop struct {
	lift    float64 // raised by
	sx, sy  float64 // squash and stretch, 0 for 1
	eyes    string  // "" (open), "wink" or "happy"
	sparkle float64 // sparkles, 0 to 1
	spin    float64 // turn about the middle
	tilt    float64 // turn about the bottom tip
}

func hopSheet() Sheet {
	happy := func(tilt float64) hop { return hop{tilt: tilt, eyes: "happy"} }
	return sheet("hop.svg", hop.draw, hop{}, []clip[hop]{
		// Crouches, hops up with a wink and sparkles, lands with a squash.
		{"hop", []hop{{sx: 1.14, sy: 0.84}, {lift: 8, sx: 0.90, sy: 1.10}, {lift: 16, sx: 0.96, sy: 1.04, eyes: "wink", sparkle: 0.5},
			{lift: 19, eyes: "wink", sparkle: 1}, {lift: 16, sx: 0.98, sy: 1.02, eyes: "wink", sparkle: 0.6},
			{lift: 7, sx: 0.92, sy: 1.08}, {sx: 1.16, sy: 0.82}, {sx: 0.96, sy: 1.05}, {sx: 1.02, sy: 0.98}}},
		// Jumps and spins once around.
		{"spin", []hop{{sx: 1.14, sy: 0.84}, {lift: 10, sx: 0.92, sy: 1.08, spin: 30}, {lift: 17, spin: 90, eyes: "happy"},
			{lift: 19, spin: 160, eyes: "happy"}, {lift: 19, spin: 230, eyes: "happy"}, {lift: 15, spin: 300, eyes: "happy"},
			{lift: 7, spin: 345, sx: 0.94, sy: 1.06}, {sx: 1.16, sy: 0.82, eyes: "happy", sparkle: 1},
			{sx: 0.97, sy: 1.04, eyes: "happy", sparkle: 0.5}, {}}},
		// Wobbles from side to side, eyes closed with joy.
		{"wobble", []hop{happy(-12), happy(-6), happy(6), happy(12), happy(6), happy(-6), happy(-12), happy(-6),
			happy(6), happy(12), happy(6), {}}},
	})
}

func (h hop) draw() string {
	sx, sy := h.sx, h.sy
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	var s svg
	// Squash and stretch about the bottom, tilt about the bottom tip, spin
	// about the middle, then lift.
	s.add(`<g transform="translate(50 %v) scale(%.3f %.3f) translate(-50 -91) rotate(%v 50 91) rotate(%v 50 52)">`,
		hopGround-h.lift, hopBase*sx, hopBase*sy, h.tilt, h.spin)
	s.add(`<polygon points="%s" fill="#f5b800" stroke="%s" stroke-width="5" stroke-linejoin="round"/>`, hopStar, hopInk)
	s.add(`<polygon points="50,20 54,36 50,42 46,36" fill="#fff6c8" opacity="0.75"/>`)
	// The face.
	const arc = `fill="none" stroke="` + hopInk + `" stroke-width="3.2" stroke-linecap="round"`
	switch h.eyes {
	case "happy":
		s.add(`<path d="M37 50Q42 43 47 50M53 50Q58 43 63 50" %s/>`, arc)
	case "wink":
		s.add(`<ellipse cx="42" cy="49" rx="3.6" ry="5" fill="%s"/>`, hopInk)
		s.add(`<path d="M53 50Q58 44 63 50" %s/>`, arc)
	default:
		s.add(`<ellipse cx="42" cy="49" rx="3.6" ry="5" fill="%s"/>`, hopInk)
		s.add(`<ellipse cx="58" cy="49" rx="3.6" ry="5" fill="%s"/>`, hopInk)
	}
	s.add(`<path d="M43 58Q50 66 57 58" %s/>`, arc)
	s.add(`<ellipse cx="35" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>` +
		`<ellipse cx="65" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>`)
	s.add("</g>")
	if a := h.sparkle; a != 0 {
		for _, p := range [][3]float64{{86, 16, 1}, {14, 26, 0.7}} {
			s.add(`<path transform="translate(%v %v) scale(%.2f)" `+
				`d="M0 -12L2.6 -2.6 12 0 2.6 2.6 0 12-2.6 2.6-12 0-2.6-2.6Z" `+
				`fill="#fff" stroke="%s" stroke-width="1.5" opacity="%v"/>`, p[0], p[1], p[2]*(0.5+0.5*a), hopInk, a)
		}
	}
	if h.lift > 10 {
		// A small shadow left on the ground.
		s.add(`<ellipse cx="50" cy="96" rx="%.1f" ry="2.5" fill="#000" opacity="0.18"/>`, 18-h.lift*0.4)
	}
	return s.String()
}
