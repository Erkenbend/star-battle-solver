# Writes the animated sprite sheet symbols/cat.svg to stdout: 12 frames of a
# cat face, side by side, 100×100 each. It twitches an ear, blinks, sticks
# its tongue out ("mlem") and grins with closed eyes. Frame 0 is the pose
# that stays on the board.
import sys

F = 12
INK = "#4a2a12"
FUR = "#f4a340"
STRIPE = "#d17a26"
CHEEK = "#fff3e0"
PINK = "#f28aa0"
TONGUE = "#ff6f8e"

# Per frame: (right ear angle, eyes, tongue length, head tilt)
# eyes: "open", "shut" (blink) or "happy" (^ ^)
FRAMES = [
    (0, "open", 0, 0),
    (-26, "open", 0, 0),
    (0, "open", 0, 0),
    (0, "shut", 0, 0),
    (0, "open", 0, 0),
    (0, "open", 5, 0),
    (0, "shut", 10, -4),
    (0, "shut", 10, -4),
    (0, "open", 5, 0),
    (0, "happy", 0, 6),
    (-12, "happy", 0, 6),
    (0, "open", 0, 0),
]
assert len(FRAMES) == F


def ear(points, inner, rot, pivot):
    t = f' transform="rotate({rot} {pivot[0]} {pivot[1]})"' if rot else ""
    p = " ".join(f"{x},{y}" for x, y in points)
    q = " ".join(f"{x},{y}" for x, y in inner)
    return (f'<g{t}><polygon points="{p}" fill="{FUR}" stroke="{INK}" stroke-width="4.5" stroke-linejoin="round"/>'
            f'<polygon points="{q}" fill="{PINK}"/></g>')


def eyes(kind):
    if kind == "open":
        return "".join(
            f'<ellipse cx="{x}" cy="55" rx="5" ry="7" fill="{INK}"/><circle cx="{x + 1.6}" cy="52.4" r="2" fill="#fff"/>'
            for x in (36, 64))
    d = "Q{} 60 {} 56" if kind == "shut" else "Q{} 49 {} 57"
    y0 = 56 if kind == "shut" else 57
    return "".join(
        f'<path d="M{x - 6} {y0} {d.format(x, x + 6)}" fill="none" stroke="{INK}" stroke-width="3.5" stroke-linecap="round"/>'
        for x in (36, 64))


def frame(ear_rot, eye_kind, tongue, tilt):
    s = []
    s.append(f'<g transform="rotate({tilt} 50 62)">')
    s.append(ear([(14, 50), (20, 10), (46, 32)], [(21, 40), (24, 20), (37, 33)], 0, (0, 0)))
    s.append(ear([(86, 50), (80, 10), (54, 32)], [(79, 40), (76, 20), (63, 33)], ear_rot, (72, 38)))
    s.append(f'<ellipse cx="50" cy="60" rx="39" ry="31" fill="{FUR}" stroke="{INK}" stroke-width="4.5"/>')
    # Tabby stripes on the forehead.
    s.append(f'<path d="M50 30V40M41 31 43 39M59 31 57 39" stroke="{STRIPE}" stroke-width="3.5" stroke-linecap="round"/>')
    s.append(f'<ellipse cx="25" cy="69" rx="6" ry="3.5" fill="{PINK}" opacity="0.55"/>'
             f'<ellipse cx="75" cy="69" rx="6" ry="3.5" fill="{PINK}" opacity="0.55"/>')
    s.append(f'<ellipse cx="50" cy="73" rx="15" ry="10" fill="{CHEEK}"/>')
    s.append(eyes(eye_kind))
    if tongue:
        s.append(f'<path d="M45 72H55V{72 + tongue}A5 5 0 0 1 45 {72 + tongue}Z" fill="{TONGUE}" stroke="{INK}" stroke-width="2.5" stroke-linejoin="round"/>'
                 f'<path d="M50 74V{70 + tongue}" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>')
    s.append(f'<path d="M46 65H54L50 70Z" fill="{PINK}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>')
    s.append(f'<path d="M50 70Q50 76 44 75M50 70Q50 76 56 75" fill="none" stroke="{INK}" stroke-width="2.5" stroke-linecap="round"/>')
    s.append(f'<path d="M22 66 6 62M22 71 6 73M78 66 94 62M78 71 94 73" stroke="{INK}" stroke-width="2" stroke-linecap="round"/>')
    s.append("</g>")
    return "".join(s)


out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {F*100} 100" width="{F*100}" height="100">']
for i, f in enumerate(FRAMES):
    out.append(f'<g transform="translate({i*100} 0)">{frame(*f)}</g>')
out.append("</svg>")
sys.stdout.write("\n".join(out) + "\n")
