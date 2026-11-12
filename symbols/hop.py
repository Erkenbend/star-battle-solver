# Writes the animated sprite sheet symbols/hop.svg to stdout (see sheet.py):
# a smiling star that hops with a wink, jumps and spins, or wobbles happily.
from sheet import sheet

INK = "#5c3d00"
pts = [(50, 6), (61.5, 37.2), (94.7, 38.5), (68.5, 59), (77.6, 91), (50, 72.5), (22.4, 91), (31.5, 59), (5.3, 38.5), (38.5, 37.2)]
poly = " ".join(f"{x},{y}" for x, y in pts)
BASE = 0.86  # star size at rest, so jumps stay inside the box
GROUND = 94

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Crouches, hops up with a wink and sparkles, lands with a squash.
    "hop": [dict(sx=1.14, sy=0.84), dict(lift=8, sx=0.90, sy=1.10), dict(lift=16, sx=0.96, sy=1.04, eyes="wink", sparkle=0.5),
            dict(lift=19, eyes="wink", sparkle=1), dict(lift=16, sx=0.98, sy=1.02, eyes="wink", sparkle=0.6),
            dict(lift=7, sx=0.92, sy=1.08), dict(sx=1.16, sy=0.82), dict(sx=0.96, sy=1.05), dict(sx=1.02, sy=0.98)],
    # Jumps and spins once around.
    "spin": [dict(sx=1.14, sy=0.84), dict(lift=10, sx=0.92, sy=1.08, spin=30), dict(lift=17, spin=90, eyes="happy"),
             dict(lift=19, spin=160, eyes="happy"), dict(lift=19, spin=230, eyes="happy"), dict(lift=15, spin=300, eyes="happy"),
             dict(lift=7, spin=345, sx=0.94, sy=1.06), dict(sx=1.16, sy=0.82, eyes="happy", sparkle=1),
             dict(sx=0.97, sy=1.04, eyes="happy", sparkle=0.5), {}],
    # Wobbles from side to side, eyes closed with joy.
    "wobble": [dict(tilt=-12, eyes="happy"), dict(tilt=-6, eyes="happy"), dict(tilt=6, eyes="happy"), dict(tilt=12, eyes="happy"),
               dict(tilt=6, eyes="happy"), dict(tilt=-6, eyes="happy"), dict(tilt=-12, eyes="happy"), dict(tilt=-6, eyes="happy"),
               dict(tilt=6, eyes="happy"), dict(tilt=12, eyes="happy"), dict(tilt=6, eyes="happy"), {}],
}


def face(eyes):
    s = []
    if eyes == "happy":
        s.append(f'<path d="M37 50Q42 43 47 50M53 50Q58 43 63 50" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
    else:
        s.append(f'<ellipse cx="42" cy="49" rx="3.6" ry="5" fill="{INK}"/>')
        if eyes == "wink":
            s.append(f'<path d="M53 50Q58 44 63 50" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
        else:
            s.append(f'<ellipse cx="58" cy="49" rx="3.6" ry="5" fill="{INK}"/>')
    s.append(f'<path d="M43 58Q50 66 57 58" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
    s.append('<ellipse cx="35" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>'
             '<ellipse cx="65" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>')
    return "".join(s)


def frame(lift=0, sx=1, sy=1, eyes="open", sparkle=0, spin=0, tilt=0):
    s = []
    # Squash and stretch about the bottom, tilt about the bottom tip, spin
    # about the middle, then lift.
    s.append(f'<g transform="translate(50 {GROUND - lift}) scale({BASE * sx:.3f} {BASE * sy:.3f}) translate(-50 -91)'
             f' rotate({tilt} 50 91) rotate({spin} 50 52)">')
    s.append(f'<polygon points="{poly}" fill="#f5b800" stroke="{INK}" stroke-width="5" stroke-linejoin="round"/>')
    s.append('<polygon points="50,20 54,36 50,42 46,36" fill="#fff6c8" opacity="0.75"/>')
    s.append(face(eyes))
    s.append("</g>")
    if sparkle:
        for x, y, k in ((86, 16, 1), (14, 26, 0.7)):
            s.append(f'<path transform="translate({x} {y}) scale({k * (0.5 + 0.5 * sparkle):.2f})" '
                     f'd="M0 -12L2.6 -2.6 12 0 2.6 2.6 0 12-2.6 2.6-12 0-2.6-2.6Z" '
                     f'fill="#fff" stroke="{INK}" stroke-width="1.5" opacity="{sparkle}"/>')
    if lift > 10:
        # A small shadow left on the ground.
        s.append(f'<ellipse cx="50" cy="96" rx="{18 - lift * 0.4:.1f}" ry="2.5" fill="#000" opacity="0.18"/>')
    return "".join(s)


sheet(frame, REST, CLIPS)
