# Writes the animated sprite sheet symbols/dog.svg to stdout (see sheet.py):
# a dog face that tilts its head at you, pants with flapping ears, or barks.
from sheet import sheet

INK = "#3a2410"
FUR = "#e3ad6e"
EAR = "#8a5a32"
PATCH = "#c98d4f"
MUZZLE = "#fff4e6"
TONGUE = "#ff6f8e"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Tilts its head, a question mark appears.
    "tilt": [dict(tilt=8, swing=6), dict(tilt=16, swing=10, tongue=4, q=0.5), dict(tilt=18, swing=8, tongue=4, q=1),
             dict(tilt=18, swing=8, tongue=4, q=1), dict(tilt=18, swing=8, tongue=4, q=1), dict(tilt=18, swing=8, tongue=4, q=1),
             dict(tilt=10, swing=2, q=0.5), {}, {}],
    # Pants happily, ears flapping.
    "pant": [dict(swing=-8, tongue=12, bob=-2), dict(swing=8, tongue=7, bob=1),
             dict(swing=-8, tongue=12, bob=-2), dict(swing=8, tongue=7, bob=1), dict(swing=-4, tongue=10, bob=-1), {}],
    # Barks twice: "wuff wuff".
    "bark": [dict(bob=-1, bark=0.5, swing=4), dict(bob=-3, bark=1, swing=10, lines=1), dict(bob=-3, bark=1, swing=10, lines=0.6),
             dict(bark=0.3), {}, dict(bob=-1, bark=0.5, swing=4), dict(bob=-3, bark=1, swing=10, lines=1),
             dict(bob=-3, bark=1, swing=10, lines=0.6), dict(bark=0.3), {}, {}],
}


def ears(swing):
    left = "M27 26C10 24 2 44 7 64C10 76 22 76 25 62Z"
    right = "M73 26C90 24 98 44 93 64C90 76 78 76 75 62Z"
    st = f'fill="{EAR}" stroke="{INK}" stroke-width="4.5" stroke-linejoin="round"'
    return (f'<path d="{left}" {st} transform="rotate({swing} 26 28)"/>'
            f'<path d="{right}" {st} transform="rotate({-swing} 74 28)"/>')


def question(a):
    if not a:
        return ""
    return (f'<g opacity="{a}" fill="none" stroke="{INK}" stroke-width="4" stroke-linecap="round">'
            f'<path d="M80 14C80 5 94 5 94 13C94 20 87 20 87 27"/></g>'
            f'<circle cx="87" cy="34" r="2.6" fill="{INK}" opacity="{a}"/>')


def bark_lines(a):
    if not a:
        return ""
    return (f'<path d="M80 22 92 14M83 30 96 28M76 15 80 4M20 22 8 14M17 30 4 28M24 15 20 4" fill="none" stroke="{INK}" '
            f'stroke-width="3.5" stroke-linecap="round" opacity="{a}"/>')


def frame(tilt=0, swing=0, tongue=6, bob=0, q=0, bark=0, lines=0):
    s = [f'<g transform="translate(0 {bob}) rotate({tilt} 50 60)">']
    s.append(ears(swing))
    s.append(f'<ellipse cx="50" cy="54" rx="31" ry="30" fill="{FUR}" stroke="{INK}" stroke-width="4.5"/>')
    s.append(f'<ellipse cx="63" cy="45" rx="10" ry="9" fill="{PATCH}"/>')
    s.append(f'<ellipse cx="50" cy="68" rx="18" ry="13" fill="{MUZZLE}"/>')
    for x in (38, 63):
        if bark >= 1:
            # Eyes squeezed shut while barking.
            s.append(f'<path d="M{x - 5} 44 {x + 4} 47 {x - 5} 50" fill="none" stroke="{INK}" stroke-width="3" '
                     f'stroke-linecap="round" stroke-linejoin="round" transform="{"" if x < 50 else f"rotate(180 {x} 47)"}"/>')
        else:
            s.append(f'<circle cx="{x}" cy="46" r="5.2" fill="{INK}"/><circle cx="{x + 1.7}" cy="44.3" r="1.9" fill="#fff"/>')
    if bark:
        cy, rx, ry = 70 + 2 * bark, 7 + 3 * bark, 3 + 7 * bark
        s.append(f'<ellipse cx="50" cy="{cy}" rx="{rx}" ry="{ry}" fill="#7a2230" stroke="{INK}" stroke-width="2.5"/>'
                 f'<ellipse cx="50" cy="{cy + 0.55 * ry}" rx="{0.65 * rx}" ry="{0.4 * ry}" fill="{TONGUE}"/>')
    else:
        s.append(f'<path d="M45 70H55V{70 + tongue}A5 5 0 0 1 45 {70 + tongue}Z" fill="{TONGUE}" stroke="{INK}" stroke-width="2.5" stroke-linejoin="round"/>'
                 f'<path d="M50 72V{68 + tongue}" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>')
        s.append(f'<path d="M50 63V68M50 68Q44 74 39 69M50 68Q56 74 61 69" fill="none" stroke="{INK}" stroke-width="2.5" stroke-linecap="round"/>')
    s.append(f'<ellipse cx="50" cy="59" rx="7.5" ry="5.5" fill="{INK}"/><ellipse cx="48" cy="57.4" rx="2.6" ry="1.5" fill="#fff" opacity="0.8"/>')
    s.append("</g>")
    s.append(question(q))
    s.append(bark_lines(lines))
    return "".join(s)


sheet(frame, REST, CLIPS)
