package sprite

import "math"

// moon.svg: a sleepy moon that snores Zzz, blows a snot bubble until it pops,
// or wakes up for a moment and nods off again.

const (
	moonInk    = "#4a3a12"
	moonFace   = "#fff1a8"
	moonCrater = "#efd97a"
	moonCap    = "#6a7fd8"
	moonBubble = "#cdeeff"
)

// moon is a frame of the moon, the zero value the resting pose.
type moon struct {
	breath float64 // swelling, 0 to 1
	zzz    bool    // Zs float up, at phase z
	z      float64
	bubble float64 // size of the snot bubble, 0 to about 1
	pop    float64 // opacity of the pop lines
	eyes   string  // "" (shut), "open" or "half"
	look   float64 // pupils to the side, -1 to 1
	yawn   float64 // 0 to 1
	lift   float64 // raised by
}

func moonSheet() Sheet {
	zzz := func(breath, z float64) moon { return moon{breath: breath, zzz: true, z: z} }
	return sheet("moon.svg", moon.draw, moon{}, []clip[moon]{
		// Breathes, a Z floats up after another.
		{"zzz", []moon{zzz(0.5, 0), zzz(1, 0.15), zzz(1, 0.3), zzz(0.5, 0.45),
			zzz(0, 0.6), zzz(0.5, 0.75), zzz(1, 0.9), {breath: 0.5}, {}}},
		// A snot bubble grows with each breath and pops; it startles a little.
		{"bubble", []moon{{bubble: 0.25, breath: 0.5}, {bubble: 0.4, breath: 1}, {bubble: 0.3}, {bubble: 0.55, breath: 0.5},
			{bubble: 0.75, breath: 1}, {bubble: 0.6}, {bubble: 0.9, breath: 0.5}, {bubble: 1.05, breath: 1},
			{pop: 1, eyes: "open", lift: 3}, {pop: 0.5, eyes: "open", lift: 1}, {eyes: "open"}, {eyes: "half"}, {}, {}}},
		// Opens an eye, looks around, yawns and dozes off again.
		{"wake", []moon{{eyes: "half"}, {eyes: "open"}, {eyes: "open", look: -1}, {eyes: "open", look: -1}, {eyes: "open", look: 1},
			{eyes: "open", look: 1}, {yawn: 0.6}, {yawn: 1}, {yawn: 1},
			{yawn: 0.4}, {eyes: "half"}, {}}},
	})
}

func (m moon) draw() string {
	var s svg
	s.add(`<g transform="translate(0 %v) translate(46 56) scale(%v) translate(-46 -56)">`, -m.lift, 1+0.03*m.breath)
	s.add(`<circle cx="46" cy="56" r="36" fill="%s" stroke="%s" stroke-width="4.5"/>`, moonFace, moonInk)
	s.add(`<circle cx="24" cy="44" r="5" fill="%s"/><circle cx="66" cy="76" r="6" fill="%s"/>`+
		`<circle cx="30" cy="78" r="3.5" fill="%s"/>`, moonCrater, moonCrater, moonCrater)
	// A nightcap with a bobble.
	s.add(`<path d="M14 40C18 18 40 8 62 14C74 18 84 24 92 34L84 40C76 30 62 26 50 26C34 26 22 32 14 40Z" fill="%s" `+
		`stroke="%s" stroke-width="4" stroke-linejoin="round"/>`, moonCap, moonInk)
	s.add(`<circle cx="90" cy="40" r="6" fill="#fff" stroke="%s" stroke-width="3"/>`, moonInk)
	for _, x := range []float64{34, 58} {
		switch m.eyes {
		case "open":
			s.add(`<circle cx="%v" cy="52" r="5" fill="#fff" stroke="%s" stroke-width="2.5"/>`+
				`<circle cx="%v" cy="53" r="2.6" fill="%s"/>`, x, moonInk, x+2*m.look, moonInk)
		case "half":
			s.add(`<path d="M%v 52H%v" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+
				`<path d="M%v 53Q%v 57 %v 53Z" fill="%s"/>`, x-6, x+6, moonInk, x-4, x, x+4, moonInk)
		default:
			s.add(`<path d="M%v 52Q%v 58 %v 52" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, x-6, x, x+6, moonInk)
		}
	}
	s.add(`<ellipse cx="26" cy="62" rx="5" ry="3" fill="#ffab8a" opacity="0.55"/>` +
		`<ellipse cx="66" cy="62" rx="5" ry="3" fill="#ffab8a" opacity="0.55"/>`)
	switch {
	case m.yawn != 0:
		s.add(`<ellipse cx="46" cy="69" rx="%v" ry="%v" fill="#7a2230" stroke="%s" stroke-width="2.5"/>`, 4+3*m.yawn, 2+6*m.yawn, moonInk)
	case m.pop != 0:
		s.add(`<ellipse cx="46" cy="68" rx="3.5" ry="4" fill="#7a2230" stroke="%s" stroke-width="2"/>`, moonInk)
	default:
		s.add(`<path d="M41 68Q46 71 51 68" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, moonInk)
	}
	s.add("</g>")
	if m.bubble != 0 {
		r := 3 + 12*m.bubble
		s.add(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" fill-opacity="0.75" stroke="%s" stroke-width="2"/>`+
			`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="#fff"/>`,
			52+0.8*r, 62+0.4*r, r, moonBubble, moonInk, 52+0.5*r, 62+0.05*r, 0.25*r)
	}
	if m.pop != 0 {
		s.add(`<path d="M64 60 72 54M68 68 78 68M64 76 72 82M58 56 58 48" stroke="%s" stroke-width="2.5" `+
			`stroke-linecap="round" opacity="%v"/>`, moonInk, m.pop)
	}
	if m.zzz {
		// Each Z rises and grows to the top right, the next one follows.
		for _, d := range []float64{0, 0.5} {
			t := math.Mod(m.z+d, 1)
			k := 0.6 + 0.8*t
			s.add(`<path transform="translate(%v %v) scale(%v)" d="M-5 -5H5L-5 5H5" fill="none" stroke="%s" `+
				`stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round" opacity="%.2f"/>`,
				72+18*t, 30-22*t, k, moonInk, 3/k, min(1, 3*(1-t)))
		}
	}
	return s.String()
}
