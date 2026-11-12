# Writes the animated sprite sheet symbols/moon.svg to stdout (see sheet.py):
# a sleepy moon that snores Zzz, blows a snot bubble until it pops, or wakes
# up for a moment and nods off again.
from sheet import sheet

INK = "#4a3a12"
MOON = "#fff1a8"
CRATER = "#efd97a"
CAP = "#6a7fd8"
BUBBLE = "#cdeeff"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Breathes, a Z floats up after another.
    "zzz": [dict(breath=0.5, z=0.0), dict(breath=1, z=0.15), dict(breath=1, z=0.3), dict(breath=0.5, z=0.45),
            dict(breath=0, z=0.6), dict(breath=0.5, z=0.75), dict(breath=1, z=0.9), dict(breath=0.5), {}],
    # A snot bubble grows with each breath and pops; it startles a little.
    "bubble": [dict(bubble=0.25, breath=0.5), dict(bubble=0.4, breath=1), dict(bubble=0.3), dict(bubble=0.55, breath=0.5),
               dict(bubble=0.75, breath=1), dict(bubble=0.6), dict(bubble=0.9, breath=0.5), dict(bubble=1.05, breath=1),
               dict(pop=1, eyes="open", lift=3), dict(pop=0.5, eyes="open", lift=1), dict(eyes="open"), dict(eyes="half"), {}, {}],
    # Opens an eye, looks around, yawns and dozes off again.
    "wake": [dict(eyes="half"), dict(eyes="open"), dict(eyes="open", look=-1), dict(eyes="open", look=-1), dict(eyes="open", look=1),
             dict(eyes="open", look=1), dict(eyes="shut", yawn=0.6), dict(eyes="shut", yawn=1), dict(eyes="shut", yawn=1),
             dict(eyes="shut", yawn=0.4), dict(eyes="half"), {}],
}


def zee(x, y, k, a):
    return (f'<path transform="translate({x} {y}) scale({k})" d="M-5 -5H5L-5 5H5" fill="none" stroke="{INK}" '
            f'stroke-width="{3 / k:.2f}" stroke-linecap="round" stroke-linejoin="round" opacity="{a:.2f}"/>')


def frame(breath=0, z=None, bubble=0, pop=0, eyes="shut", look=0, yawn=0, lift=0):
    s = [f'<g transform="translate(0 {-lift}) translate(46 56) scale({1 + 0.03 * breath}) translate(-46 -56)">']
    s.append(f'<circle cx="46" cy="56" r="36" fill="{MOON}" stroke="{INK}" stroke-width="4.5"/>')
    s.append(f'<circle cx="24" cy="44" r="5" fill="{CRATER}"/><circle cx="66" cy="76" r="6" fill="{CRATER}"/>'
             f'<circle cx="30" cy="78" r="3.5" fill="{CRATER}"/>')
    # A nightcap with a bobble.
    s.append(f'<path d="M14 40C18 18 40 8 62 14C74 18 84 24 92 34L84 40C76 30 62 26 50 26C34 26 22 32 14 40Z" fill="{CAP}" '
             f'stroke="{INK}" stroke-width="4" stroke-linejoin="round"/>')
    s.append(f'<circle cx="90" cy="40" r="6" fill="#fff" stroke="{INK}" stroke-width="3"/>')
    for x in (34, 58):
        if eyes == "open":
            s.append(f'<circle cx="{x}" cy="52" r="5" fill="#fff" stroke="{INK}" stroke-width="2.5"/>'
                     f'<circle cx="{x + 2 * look}" cy="53" r="2.6" fill="{INK}"/>')
        elif eyes == "half":
            s.append(f'<path d="M{x - 6} 52H{x + 6}" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>'
                     f'<path d="M{x - 4} 53Q{x} 57 {x + 4} 53Z" fill="{INK}"/>')
        else:
            s.append(f'<path d="M{x - 6} 52Q{x} 58 {x + 6} 52" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    s.append('<ellipse cx="26" cy="62" rx="5" ry="3" fill="#ffab8a" opacity="0.55"/>'
             '<ellipse cx="66" cy="62" rx="5" ry="3" fill="#ffab8a" opacity="0.55"/>')
    if yawn:
        s.append(f'<ellipse cx="46" cy="69" rx="{4 + 3 * yawn}" ry="{2 + 6 * yawn}" fill="#7a2230" stroke="{INK}" stroke-width="2.5"/>')
    elif pop:
        s.append(f'<ellipse cx="46" cy="68" rx="3.5" ry="4" fill="#7a2230" stroke="{INK}" stroke-width="2"/>')
    else:
        s.append(f'<path d="M41 68Q46 71 51 68" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    s.append("</g>")
    if bubble:
        r = 3 + 12 * bubble
        s.append(f'<circle cx="{52 + 0.8 * r:.1f}" cy="{62 + 0.4 * r:.1f}" r="{r:.1f}" fill="{BUBBLE}" fill-opacity="0.75" stroke="{INK}" stroke-width="2"/>'
                 f'<circle cx="{52 + 0.5 * r:.1f}" cy="{62 + 0.05 * r:.1f}" r="{0.25 * r:.1f}" fill="#fff"/>')
    if pop:
        s.append(f'<path d="M64 60 72 54M68 68 78 68M64 76 72 82M58 56 58 48" stroke="{INK}" stroke-width="2.5" '
                 f'stroke-linecap="round" opacity="{pop}"/>')
    if z is not None:
        # Each Z rises and grows to the top right, the next one follows.
        for d in (0, 0.5):
            t = (z + d) % 1
            s.append(zee(72 + 18 * t, 30 - 22 * t, 0.6 + 0.8 * t, min(1, 3 * (1 - t))))
    return "".join(s)


sheet(frame, REST, CLIPS)
