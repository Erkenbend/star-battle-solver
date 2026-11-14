package sprite

// frog.svg: a frog that catches a fly with its tongue, croaks, or rolls its
// eyes.

const (
	frogInk    = "#1f3d14"
	frogSkin   = "#7cc95a"
	frogBelly  = "#d8f0b0"
	frogTongue = "#ff6f8e"
	frogSac    = "#e9f7cf"
)

// pt is a point; as the tongue's tip or the fly's place, the zero point
// means there is none.
type pt struct{ x, y float64 }

// frog is a frame of the frog, the zero value the resting pose.
type frog struct {
	look   pt     // direction of the pupils, each -1 to 1
	eyes   string // "" (open), "happy", "shut", "left" or "right" (one shut)
	tongue pt     // tip of the tongue
	fly    pt     // where the fly is
	cheeks float64
	sac    float64 // throat sac, 0 to 1
	lines  float64 // opacity of the croak lines
}

func frogSheet() Sheet {
	look := func(x, y float64) frog { return frog{look: pt{x, y}} }
	return sheet("frog.svg", frog.draw, frog{}, []clip[frog]{
		// A fly buzzes by, the tongue snaps it, gulp.
		{"fly", []frog{{fly: pt{90, 14}, look: pt{1, -1}}, {fly: pt{80, 9}, look: pt{1, -1}}, {fly: pt{88, 20}, look: pt{1, -0.5}},
			{fly: pt{82, 14}, look: pt{1, -1}, tongue: pt{82, 14}}, {tongue: pt{66, 44}, fly: pt{66, 44}, look: pt{1, -1}},
			{cheeks: 1, eyes: "happy"}, {cheeks: 0.6, eyes: "happy"}, {eyes: "happy"}, {}}},
		// Croaks twice with a throat sac.
		{"croak", []frog{{sac: 0.4}, {sac: 1, lines: 1, eyes: "shut"}, {sac: 1, lines: 0.6, eyes: "shut"}, {sac: 0.3}, {},
			{sac: 0.4}, {sac: 1, lines: 1, eyes: "shut"}, {sac: 1, lines: 0.6, eyes: "shut"}, {sac: 0.3}, {}, {}}},
		// Rolls its eyes round, then blinks one after the other.
		{"eyes", []frog{look(1, 0), look(0.7, -0.7), look(0, -1), look(-0.7, -0.7), look(-1, 0),
			look(-0.7, 0.7), look(0, 1), {}, {eyes: "left"}, {}, {eyes: "right"}, {}}},
	})
}

func (f frog) draw() string {
	var s svg
	s.add(`<ellipse cx="50" cy="62" rx="%v" ry="28" fill="%s" stroke="%s" stroke-width="4.5"/>`, 40+3*f.cheeks, frogSkin, frogInk)
	s.add(`<ellipse cx="50" cy="76" rx="24" ry="11" fill="%s"/>`, frogBelly)
	if f.sac != 0 {
		// The sac in front of the belly, below the mouth.
		s.add(`<circle cx="50" cy="%v" r="%v" fill="%s" stroke="%s" stroke-width="3"/>`, 80+4*f.sac, 6+10*f.sac, frogSac, frogInk)
	}
	const arc = `fill="none" stroke="` + frogInk + `" stroke-width="3.5" stroke-linecap="round"`
	for _, x := range []float64{30, 70} {
		s.add(`<circle cx="%v" cy="34" r="15" fill="%s" stroke="%s" stroke-width="4.5"/>`, x, frogSkin, frogInk)
		shut := f.eyes == "shut" || f.eyes == "left" && x < 50 || f.eyes == "right" && x > 50
		switch {
		case f.eyes == "happy":
			s.add(`<path d="M%v 37Q%v 28 %v 37" %s/>`, x-8, x, x+8, arc)
		case shut:
			s.add(`<path d="M%v 34Q%v 40 %v 34" %s/>`, x-8, x, x+8, arc)
		default:
			dx, dy := f.look.x, f.look.y
			s.add(`<circle cx="%v" cy="34" r="9.5" fill="#fff"/>`+
				`<circle cx="%.1f" cy="%.1f" r="5" fill="%s"/>`+
				`<circle cx="%.1f" cy="%.1f" r="1.6" fill="#fff"/>`,
				x, x+3.5*dx, 34+3.5*dy, frogInk, x+3.5*dx+1.6, 34+3.5*dy-1.6)
		}
	}
	s.add(`<circle cx="45" cy="56" r="1.6" fill="%s"/><circle cx="55" cy="56" r="1.6" fill="%s"/>`, frogInk, frogInk)
	s.add(`<ellipse cx="21" cy="66" rx="6" ry="3.5" fill="#ff8a9a" opacity="0.55"/>` +
		`<ellipse cx="79" cy="66" rx="6" ry="3.5" fill="#ff8a9a" opacity="0.55"/>`)
	const mouth = `fill="none" stroke="` + frogInk + `" stroke-width="3" stroke-linecap="round"`
	switch t := f.tongue; {
	case t != pt{}:
		s.add(`<path d="M50 68L%v %v" stroke="%s" stroke-width="8" stroke-linecap="round"/>`, t.x, t.y, frogInk)
		s.add(`<path d="M50 68L%v %v" stroke="%s" stroke-width="4.5" stroke-linecap="round"/>`, t.x, t.y, frogTongue)
		s.add(`<ellipse cx="50" cy="69" rx="8" ry="4" fill="#7a2230" stroke="%s" stroke-width="2.5"/>`, frogInk)
	case f.cheeks != 0:
		s.add(`<path d="M36 66Q50 72 64 66" %s/>`, mouth)
	default:
		s.add(`<path d="M30 64Q50 78 70 64" %s/>`, mouth)
	}
	if p := f.fly; p != (pt{}) {
		s.add(`<g transform="translate(%v %v)">`, p.x, p.y)
		s.add(`<ellipse cx="-3" cy="-5" rx="5" ry="3.5" fill="#dff4ff" stroke="%s" stroke-width="1.2" transform="rotate(-30 -3 -5)"/>`, frogInk)
		s.add(`<ellipse cx="3" cy="-5" rx="5" ry="3.5" fill="#dff4ff" stroke="%s" stroke-width="1.2" transform="rotate(30 3 -5)"/>`, frogInk)
		s.add(`<circle r="4" fill="#333"/></g>`)
	}
	if a := f.lines; a != 0 {
		s.add(`<path d="M80 80 92 76M82 88 95 88M80 96 92 99M20 80 8 76M18 88 5 88M20 96 8 99" fill="none" stroke="%s" `+
			`stroke-width="3" stroke-linecap="round" opacity="%v"/>`, frogInk, a)
	}
	return s.String()
}
