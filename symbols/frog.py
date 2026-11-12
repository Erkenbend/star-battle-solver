# Writes the animated sprite sheet symbols/frog.svg to stdout (see sheet.py):
# a frog that catches a fly with its tongue, croaks, or rolls its eyes.
from sheet import sheet

INK = "#1f3d14"
SKIN = "#7cc95a"
BELLY = "#d8f0b0"
TONGUE = "#ff6f8e"
SAC = "#e9f7cf"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # A fly buzzes by, the tongue snaps it, gulp.
    "fly": [dict(fly=(90, 14), look=(1, -1)), dict(fly=(80, 9), look=(1, -1)), dict(fly=(88, 20), look=(1, -0.5)),
            dict(fly=(82, 14), look=(1, -1), tongue=(82, 14)), dict(tongue=(66, 44), fly=(66, 44), look=(1, -1)),
            dict(cheeks=1, eyes="happy"), dict(cheeks=0.6, eyes="happy"), dict(eyes="happy"), {}],
    # Croaks twice with a throat sac.
    "croak": [dict(sac=0.4), dict(sac=1, lines=1, eyes="shut"), dict(sac=1, lines=0.6, eyes="shut"), dict(sac=0.3), {},
              dict(sac=0.4), dict(sac=1, lines=1, eyes="shut"), dict(sac=1, lines=0.6, eyes="shut"), dict(sac=0.3), {}, {}],
    # Rolls its eyes round, then blinks one after the other.
    "eyes": [dict(look=(1, 0)), dict(look=(0.7, -0.7)), dict(look=(0, -1)), dict(look=(-0.7, -0.7)), dict(look=(-1, 0)),
             dict(look=(-0.7, 0.7)), dict(look=(0, 1)), {}, dict(eyes="left"), {}, dict(eyes="right"), {}],
}


def eye(x, look, shut):
    s = [f'<circle cx="{x}" cy="34" r="15" fill="{SKIN}" stroke="{INK}" stroke-width="4.5"/>']
    if shut:
        s.append(f'<path d="M{x - 8} 34Q{x} 40 {x + 8} 34" fill="none" stroke="{INK}" stroke-width="3.5" stroke-linecap="round"/>')
    else:
        dx, dy = look
        s.append(f'<circle cx="{x}" cy="34" r="9.5" fill="#fff"/>'
                 f'<circle cx="{x + 3.5 * dx:.1f}" cy="{34 + 3.5 * dy:.1f}" r="5" fill="{INK}"/>'
                 f'<circle cx="{x + 3.5 * dx + 1.6:.1f}" cy="{34 + 3.5 * dy - 1.6:.1f}" r="1.6" fill="#fff"/>')
    return "".join(s)


def happy_eye(x):
    return (f'<circle cx="{x}" cy="34" r="15" fill="{SKIN}" stroke="{INK}" stroke-width="4.5"/>'
            f'<path d="M{x - 8} 37Q{x} 28 {x + 8} 37" fill="none" stroke="{INK}" stroke-width="3.5" stroke-linecap="round"/>')


def fly_at(x, y):
    return (f'<g transform="translate({x} {y})">'
            f'<ellipse cx="-3" cy="-5" rx="5" ry="3.5" fill="#dff4ff" stroke="{INK}" stroke-width="1.2" transform="rotate(-30 -3 -5)"/>'
            f'<ellipse cx="3" cy="-5" rx="5" ry="3.5" fill="#dff4ff" stroke="{INK}" stroke-width="1.2" transform="rotate(30 3 -5)"/>'
            f'<circle r="4" fill="#333"/></g>')


def frame(look=(0, 0), eyes="open", tongue=None, fly=None, cheeks=0, sac=0, lines=0):
    s = []
    s.append(f'<ellipse cx="50" cy="62" rx="{40 + 3 * cheeks}" ry="28" fill="{SKIN}" stroke="{INK}" stroke-width="4.5"/>')
    s.append(f'<ellipse cx="50" cy="76" rx="24" ry="11" fill="{BELLY}"/>')
    if sac:
        # The sac in front of the belly, below the mouth.
        s.append(f'<circle cx="50" cy="{80 + 4 * sac}" r="{6 + 10 * sac}" fill="{SAC}" stroke="{INK}" stroke-width="3"/>')
    if eyes == "happy":
        s.append(happy_eye(30) + happy_eye(70))
    else:
        s.append(eye(30, look, eyes in ("shut", "left")) + eye(70, look, eyes in ("shut", "right")))
    s.append(f'<circle cx="45" cy="56" r="1.6" fill="{INK}"/><circle cx="55" cy="56" r="1.6" fill="{INK}"/>')
    s.append(f'<ellipse cx="21" cy="66" rx="6" ry="3.5" fill="#ff8a9a" opacity="0.55"/>'
             f'<ellipse cx="79" cy="66" rx="6" ry="3.5" fill="#ff8a9a" opacity="0.55"/>')
    if tongue:
        x, y = tongue
        s.append(f'<path d="M50 68L{x} {y}" stroke="{INK}" stroke-width="8" stroke-linecap="round"/>'
                 f'<path d="M50 68L{x} {y}" stroke="{TONGUE}" stroke-width="4.5" stroke-linecap="round"/>'
                 f'<ellipse cx="50" cy="69" rx="8" ry="4" fill="#7a2230" stroke="{INK}" stroke-width="2.5"/>')
    elif cheeks:
        s.append(f'<path d="M36 66Q50 72 64 66" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    else:
        s.append(f'<path d="M30 64Q50 78 70 64" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    if fly:
        s.append(fly_at(*fly))
    if lines:
        s.append(f'<path d="M80 80 92 76M82 88 95 88M80 96 92 99M20 80 8 76M18 88 5 88M20 96 8 99" fill="none" stroke="{INK}" '
                 f'stroke-width="3" stroke-linecap="round" opacity="{lines}"/>')
    return "".join(s)


sheet(frame, REST, CLIPS)
