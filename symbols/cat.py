# Writes the animated sprite sheet symbols/cat.svg to stdout (see sheet.py):
# a cat face that sticks its tongue out, twitches an ear and grins, yawns,
# or falls in love.
from sheet import sheet

INK = "#4a2a12"
FUR = "#f4a340"
STRIPE = "#d17a26"
CHEEK = "#fff3e0"
PINK = "#f28aa0"
TONGUE = "#ff6f8e"
RED = "#e8395a"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Sticks the tongue out.
    "mlem": [dict(tongue=5), dict(eyes="shut", tongue=10, tilt=-4), dict(eyes="shut", tongue=10, tilt=-4),
             dict(eyes="shut", tongue=10, tilt=-4), dict(tongue=5), {}, {}, {}],
    # Twitches an ear, blinks and grins.
    "ears": [dict(ear=-26), {}, dict(ear=-26), {}, dict(eyes="shut"), {}, dict(eyes="happy", tilt=6),
             dict(eyes="happy", tilt=6, ear=-12), dict(eyes="happy", tilt=6), {}],
    # Yawns with flat ears.
    "yawn": [dict(eyes="shut"), dict(eyes="shut", gape=0.4, lift=0.5, ears=6), dict(eyes="shut", gape=0.8, lift=1, ears=12),
             dict(eyes="shut", gape=1, lift=1, ears=14), dict(eyes="shut", gape=1, lift=1, ears=14),
             dict(eyes="shut", gape=1, lift=1, ears=14), dict(eyes="shut", gape=0.6, lift=0.5, ears=8),
             dict(eyes="shut", ears=2), dict(eyes="shut"), {}, {}, {}],
    # Falls in love: heart eyes, a heart floats up.
    "love": [dict(eyes="heart", tilt=-5), dict(eyes="heart", tilt=-5, heart=0.15), dict(eyes="heart", tilt=-5, heart=0.35),
             dict(eyes="heart", tilt=-5, heart=0.55), dict(eyes="heart", tilt=-5, heart=0.75),
             dict(eyes="heart", tilt=-5, heart=0.95), dict(eyes="happy"), {}],
}


def heart_path(x, y, k):
    """A heart path centered at (x, y), k units wide."""
    h = k / 2
    return (f"M{x} {y + 0.7 * h}C{x - 1.3 * h} {y - 0.1 * h} {x - h} {y - 1.1 * h} {x} {y - 0.45 * h}"
            f"C{x + h} {y - 1.1 * h} {x + 1.3 * h} {y - 0.1 * h} {x} {y + 0.7 * h}Z")


def ear(points, inner, rot, pivot):
    t = f' transform="rotate({rot} {pivot[0]} {pivot[1]})"' if rot else ""
    p = " ".join(f"{x},{y}" for x, y in points)
    q = " ".join(f"{x},{y}" for x, y in inner)
    return (f'<g{t}><polygon points="{p}" fill="{FUR}" stroke="{INK}" stroke-width="4.5" stroke-linejoin="round"/>'
            f'<polygon points="{q}" fill="{PINK}"/></g>')


def draw_eyes(kind):
    if kind == "open":
        return "".join(
            f'<ellipse cx="{x}" cy="55" rx="5" ry="7" fill="{INK}"/><circle cx="{x + 1.6}" cy="52.4" r="2" fill="#fff"/>'
            for x in (36, 64))
    if kind == "heart":
        return "".join(f'<path d="{heart_path(x, 55, 15)}" fill="{RED}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>'
                       for x in (36, 64))
    d = "Q{} 60 {} 56" if kind == "shut" else "Q{} 49 {} 57"
    y0 = 56 if kind == "shut" else 57
    return "".join(
        f'<path d="M{x - 6} {y0} {d.format(x, x + 6)}" fill="none" stroke="{INK}" stroke-width="3.5" stroke-linecap="round"/>'
        for x in (36, 64))


def frame(ear=0, eyes="open", tongue=0, tilt=0, gape=0, lift=0, ears=0, heart=0):
    s = [f'<g transform="translate(0 {-lift}) rotate({tilt} 50 62)">']
    s.append(ear_pair(ear, ears))
    s.append(f'<ellipse cx="50" cy="60" rx="39" ry="31" fill="{FUR}" stroke="{INK}" stroke-width="4.5"/>')
    # Tabby stripes on the forehead.
    s.append(f'<path d="M50 30V40M41 31 43 39M59 31 57 39" stroke="{STRIPE}" stroke-width="3.5" stroke-linecap="round"/>')
    s.append(f'<ellipse cx="25" cy="69" rx="6" ry="3.5" fill="{PINK}" opacity="0.55"/>'
             f'<ellipse cx="75" cy="69" rx="6" ry="3.5" fill="{PINK}" opacity="0.55"/>')
    s.append(f'<ellipse cx="50" cy="73" rx="15" ry="10" fill="{CHEEK}"/>')
    s.append(draw_eyes(eyes))
    if gape:
        # Wide open mouth with fangs and the tongue at the bottom.
        cy, rx, ry = 73 + 4 * gape, 6 + 4 * gape, 2 + 9 * gape
        s.append(f'<ellipse cx="50" cy="{cy}" rx="{rx}" ry="{ry}" fill="#7a2230" stroke="{INK}" stroke-width="2.5"/>'
                 f'<ellipse cx="50" cy="{cy + 0.55 * ry}" rx="{0.7 * rx}" ry="{0.4 * ry}" fill="{TONGUE}"/>'
                 f'<path d="M{50 - 0.6 * rx} {cy - 0.85 * ry}l2 {2 + 3 * gape} 2 {-2 - 3 * gape}Z'
                 f'M{50 + 0.6 * rx} {cy - 0.85 * ry}l-2 {2 + 3 * gape}-2 {-2 - 3 * gape}Z" fill="#fff"/>')
    elif tongue:
        s.append(f'<path d="M45 72H55V{72 + tongue}A5 5 0 0 1 45 {72 + tongue}Z" fill="{TONGUE}" stroke="{INK}" stroke-width="2.5" stroke-linejoin="round"/>'
                 f'<path d="M50 74V{70 + tongue}" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>')
    s.append(f'<path d="M46 65H54L50 70Z" fill="{PINK}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>')
    if not gape:
        s.append(f'<path d="M50 70Q50 76 44 75M50 70Q50 76 56 75" fill="none" stroke="{INK}" stroke-width="2.5" stroke-linecap="round"/>')
    s.append(f'<path d="M22 66 6 62M22 71 6 73M78 66 94 62M78 71 94 73" stroke="{INK}" stroke-width="2" stroke-linecap="round"/>')
    s.append("</g>")
    if heart:
        a = min(1, 2.5 * (1 - heart))
        s.append(f'<path d="{heart_path(84 + 4 * heart, 30 - 22 * heart, 12 + 6 * heart)}" fill="{RED}" stroke="{INK}" '
                 f'stroke-width="2" stroke-linejoin="round" opacity="{a:.2f}"/>')
    return "".join(s)


def ear_pair(right, both):
    """The ears: the right one turned by right, both laid flat by both."""
    return (ear([(14, 50), (20, 10), (46, 32)], [(21, 40), (24, 20), (37, 33)], -both, (28, 38))
            + ear([(86, 50), (80, 10), (54, 32)], [(79, 40), (76, 20), (63, 33)], right + both, (72, 38)))


sheet(frame, REST, CLIPS)
