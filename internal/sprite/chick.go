package sprite

// chick.svg: a chick peeking out of its egg that peeps, wobbles the egg, or
// hops out flapping so the shell cap flies up.

const (
	chickInk   = "#5c3d00"
	chickShell = "#fdf6e3"
	chickSpot  = "#e8d9b5"
	chickBody  = "#ffd84a"
	chickBeak  = "#ff9a2e"
	chickEgg   = "M19 60L27 67 35 59 43 67 51 59 59 67 67 59 75 67 81 60C83 81 70 95 50 95C30 95 17 81 19 60Z"
	chickCap   = "M30 38L35 44 41 37 47 44 53 37 59 44 65 37 70 38C70 25 61 16 50 16C39 16 30 25 30 38Z"
)

// chick is a frame of the chick, the zero value the resting pose.
type chick struct {
	beak  bool    // open
	cap   float64 // the shell cap raised by
	notes float64 // opacity of the notes of a peep
	rock  float64 // turn of the egg
	eyes  string  // "" (open), "happy" or "shut"
	rise  float64 // the chick raised by
	wings float64 // 1 up, -1 down, 0 folded
}

func chickSheet() Sheet {
	return sheet("chick.svg", chick.draw, chick{}, []clip[chick]{
		// Peeps twice, the cap bobbing.
		{"peep", []chick{{beak: true, cap: 4, notes: 0.6}, {beak: true, cap: 6, notes: 1}, {}, {beak: true, cap: 4, notes: 0.6},
			{beak: true, cap: 6, notes: 1}, {}, {}, {}}},
		// Wobbles the egg from side to side.
		{"wobble", []chick{{rock: -10}, {rock: -14, eyes: "shut"}, {rock: -6}, {rock: 6}, {rock: 14, eyes: "shut"},
			{rock: 10}, {rock: 0}, {rock: -6}, {rock: 4}, {}}},
		// Hops out flapping, the cap flies up and lands back on the head.
		{"hatch", []chick{{rise: 4, cap: 4}, {rise: 12, cap: 14, wings: 1, eyes: "happy"}, {rise: 16, cap: 26, wings: -1, eyes: "happy", beak: true},
			{rise: 16, cap: 34, wings: 1, eyes: "happy", beak: true}, {rise: 14, cap: 30, wings: -1, eyes: "happy"},
			{rise: 8, cap: 16, wings: 1}, {rise: 2, cap: 4}, {}, {}}},
	})
}

func (c chick) draw() string {
	rise := c.rise + 5 // at rest the chick peeks out just above the rim
	var s svg
	s.add(`<g transform="rotate(%v 50 95)">`, c.rock)
	// The chick, below the egg's rim.
	s.add(`<g transform="translate(0 %v)">`, -rise)
	if c.wings != 0 {
		a := 25.0
		if c.wings > 0 {
			a = -55
		}
		for _, side := range []float64{-1, 1} {
			s.add(`<ellipse cx="%v" cy="62" rx="11" ry="6.5" fill="%s" stroke="%s" stroke-width="3.5" `+
				`transform="rotate(%v %v 62)"/>`, 50+side*21, chickBody, chickInk, side*a, 50+side*14)
		}
	}
	s.add(`<circle cx="50" cy="55" r="23" fill="%s" stroke="%s" stroke-width="4.5"/>`, chickBody, chickInk)
	const arc = `fill="none" stroke="` + chickInk + `" stroke-width="3" stroke-linecap="round"`
	for _, x := range []float64{41, 59} {
		switch c.eyes {
		case "":
			s.add(`<circle cx="%v" cy="52" r="3.6" fill="%s"/><circle cx="%v" cy="50.8" r="1.2" fill="#fff"/>`, x, chickInk, x+1.2)
		case "happy":
			s.add(`<path d="M%v 54Q%v 47 %v 54" %s/>`, x-4, x, x+4, arc)
		default:
			s.add(`<path d="M%v 52Q%v 56 %v 52" %s/>`, x-4, x, x+4, arc)
		}
	}
	s.add(`<ellipse cx="34" cy="60" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.55"/>` +
		`<ellipse cx="66" cy="60" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.55"/>`)
	const beak = `fill="` + chickBeak + `" stroke="` + chickInk + `" stroke-width="2" stroke-linejoin="round"`
	if c.beak {
		s.add(`<path d="M45 57L50 53 55 57Z" %s/><path d="M46 60L50 65 54 60Z" %s/>`, beak, beak)
	} else {
		s.add(`<path d="M45 57L50 54 55 57 50 61Z" %s/>`, beak)
	}
	s.add("</g>")
	s.add(`<path d="%s" fill="%s" stroke="%s" stroke-width="4.5" stroke-linejoin="round"/>`, chickEgg, chickShell, chickInk)
	s.add(`<ellipse cx="36" cy="80" rx="4" ry="3" fill="%s"/><ellipse cx="62" cy="84" rx="5" ry="3.5" fill="%s"/>`+
		`<ellipse cx="55" cy="73" rx="3" ry="2.2" fill="%s"/>`, chickSpot, chickSpot, chickSpot)
	s.add(`<g transform="translate(0 %v) rotate(%v 50 30)">`+
		`<path d="%s" fill="%s" stroke="%s" stroke-width="4" stroke-linejoin="round"/>`+
		`<ellipse cx="44" cy="27" rx="3" ry="2" fill="%s"/></g>`, -rise-c.cap, c.cap*0.6, chickCap, chickShell, chickInk, chickSpot)
	s.add("</g>")
	if a := c.notes; a != 0 {
		// Two little notes for the peep.
		s.add(`<g opacity="%v" fill="%s" stroke="%s" stroke-width="2.4" stroke-linecap="round">`+
			`<path d="M84 26V12L92 10V22" fill="none"/><ellipse cx="82" cy="27" rx="3.2" ry="2.4"/><ellipse cx="90" cy="23" rx="3.2" ry="2.4"/>`+
			`<path d="M16 22V10" fill="none"/><ellipse cx="14" cy="23" rx="3.2" ry="2.4"/></g>`, a, chickInk, chickInk)
	}
	return s.String()
}
