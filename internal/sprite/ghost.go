package sprite

// ghost.svg: a little ghost that shouts "boo", fades away and comes back, or
// floats about sticking its tongue out.

const (
	ghostInk    = "#2c2c4a"
	ghostBody   = "#fbfbff"
	ghostShade  = "#dcdcf2"
	ghostTongue = "#ff6f8e"
)

// ghost is a frame of the ghost, the zero value the resting pose.
type ghost struct {
	grow  float64 // size above 1
	arms  float64 // raised, 0 to 1
	mouth string  // "" (small), "o", "boo", "grin" or "bleh"
	brows bool
	eyes  string  // "" (open), "happy", "shut" or "wink"
	tilt  float64 // turn about the bottom
	alpha float64 // opacity, 0 for 1
	lift  float64 // raised by
	wave  bool    // the hem's other phase
}

func ghostSheet() Sheet {
	return sheet("ghost.svg", ghost.draw, ghost{}, []clip[ghost]{
		// Puffs up and shouts "boo", then giggles.
		{"boo", []ghost{{grow: 0.04, arms: 0.5, mouth: "o"}, {grow: 0.1, arms: 1, mouth: "boo", brows: true}, {grow: 0.12, arms: 1, mouth: "boo", brows: true},
			{grow: 0.12, arms: 1, mouth: "boo", brows: true}, {grow: 0.04, arms: 0.3}, {eyes: "happy", mouth: "grin", tilt: -6},
			{eyes: "happy", mouth: "grin", tilt: 6}, {eyes: "happy", mouth: "grin", tilt: -6}, {}}},
		// Fades away while rising and comes back.
		{"fade", []ghost{{alpha: 0.8, lift: 2, eyes: "shut"}, {alpha: 0.55, lift: 4, eyes: "shut"}, {alpha: 0.3, lift: 6, eyes: "shut"},
			{alpha: 0.12, lift: 7, eyes: "shut"}, {alpha: 0.12, lift: 7, eyes: "shut"}, {alpha: 0.35, lift: 5},
			{alpha: 0.65, lift: 3, mouth: "o"}, {alpha: 1, lift: 1, mouth: "o"}, {}, {}}},
		// Floats up and down with a waving hem, tongue out.
		{"float", []ghost{{lift: 2, wave: true}, {lift: 4, mouth: "bleh", eyes: "wink"}, {lift: 5, wave: true, mouth: "bleh", eyes: "wink"},
			{lift: 4, mouth: "bleh", eyes: "wink"}, {lift: 2, wave: true}, {lift: 0}, {lift: -1, wave: true}, {}}},
	})
}

func (g ghost) draw() string {
	alpha := g.alpha
	if alpha == 0 {
		alpha = 1
	}
	var s svg
	s.add(`<g opacity="%v" transform="translate(0 %v) rotate(%v 50 90) translate(50 90) scale(%v) translate(-50 -90)">`,
		alpha, -g.lift, g.tilt, 1+g.grow)
	for _, side := range []float64{-1, 1} {
		// Little arms, raised when spooking.
		a := side * (-10 - 70*g.arms)
		s.add(`<ellipse cx="%v" cy="60" rx="9" ry="6" fill="%s" stroke="%s" stroke-width="4" `+
			`transform="rotate(%v %v 60)"/>`, 50+side*30, ghostBody, ghostInk, a, 50+side*24)
	}
	// The hem waves by moving its points between two phases.
	hem := "L69 82 60 90 50 82 40 90 31 82Z"
	if g.wave {
		hem = "L70 88 61 82 50 89 39 82 30 88Z"
	}
	s.add(`<path d="M22 90L22 46C22 25 34 12 50 12C66 12 78 25 78 46L78 90%s" fill="%s" stroke="%s" stroke-width="4.5" stroke-linejoin="round"/>`,
		hem, ghostBody, ghostInk)
	s.add(`<path d="M66 22C73 28 74 40 74 52" fill="none" stroke="%s" stroke-width="5" stroke-linecap="round"/>`, ghostShade)
	const arc = `fill="none" stroke="` + ghostInk + `" stroke-width="3.2" stroke-linecap="round"`
	for _, x := range []float64{40, 60} {
		switch {
		case g.eyes == "happy":
			s.add(`<path d="M%v 45Q%v 38 %v 45" %s/>`, x-5, x, x+5, arc)
		case g.eyes == "shut" || g.eyes == "wink" && x == 60:
			s.add(`<path d="M%v 43Q%v 48 %v 43" %s/>`, x-5, x, x+5, arc)
		default:
			s.add(`<ellipse cx="%v" cy="43" rx="4" ry="5.5" fill="%s"/><circle cx="%v" cy="41" r="1.5" fill="#fff"/>`, x, ghostInk, x+1.4)
		}
	}
	if g.brows {
		s.add(`<path d="M33 33 45 37M67 33 55 37" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, ghostInk)
	}
	s.add(`<ellipse cx="32" cy="52" rx="4.5" ry="2.8" fill="#ff9ab0" opacity="0.6"/>` +
		`<ellipse cx="68" cy="52" rx="4.5" ry="2.8" fill="#ff9ab0" opacity="0.6"/>`)
	switch g.mouth {
	case "boo":
		s.add(`<ellipse cx="50" cy="62" rx="9" ry="11" fill="#4a2440" stroke="%s" stroke-width="3"/>`+
			`<ellipse cx="50" cy="68" rx="5.5" ry="3.5" fill="%s"/>`, ghostInk, ghostTongue)
	case "o":
		s.add(`<ellipse cx="50" cy="56" rx="3.5" ry="4.5" fill="#4a2440" stroke="%s" stroke-width="2"/>`, ghostInk)
	case "grin":
		s.add(`<path d="M42 54Q50 64 58 54Z" fill="#4a2440" stroke="%s" stroke-width="2.5" stroke-linejoin="round"/>`, ghostInk)
	case "bleh":
		s.add(`<path d="M44 55Q50 58 56 55" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+
			`<path d="M47 56H55V61A4 4 0 0 1 47 61Z" fill="%s" stroke="%s" stroke-width="2.2" stroke-linejoin="round"/>`,
			ghostInk, ghostTongue, ghostInk)
	default:
		s.add(`<path d="M46 55Q50 59 54 55" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, ghostInk)
	}
	s.add("</g>")
	if g.mouth == "boo" {
		s.add(`<path d="M84 18 94 12M86 28 98 28M14 18 4 12M12 28 0 28" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, ghostInk)
	}
	return s.String()
}
