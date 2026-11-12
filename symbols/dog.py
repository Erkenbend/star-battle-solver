# Writes the animated sprite sheet symbols/dog.svg to stdout: 12 frames of a
# dog face, side by side, 100×100 each. It tilts its head at you with a
# question mark, then pants happily with flapping ears. Frame 0 is the pose
# that stays on the board.
import sys

F = 12
INK = "#3a2410"
FUR = "#e3ad6e"
EAR = "#8a5a32"
PATCH = "#c98d4f"
MUZZLE = "#fff4e6"
TONGUE = "#ff6f8e"

# Per frame: (head tilt, ear swing, tongue length, bob, question mark)
FRAMES = [
    (0, 0, 6, 0, 0),
    (8, 6, 6, 0, 0),
    (16, 10, 4, 0, 0.5),
    (18, 8, 4, 0, 1),
    (18, 8, 4, 0, 1),
    (10, 2, 6, 0, 0.5),
    (0, -8, 12, -2, 0),
    (0, 8, 7, 1, 0),
    (0, -8, 12, -2, 0),
    (0, 8, 7, 1, 0),
    (0, -4, 10, -1, 0),
    (0, 0, 6, 0, 0),
]
assert len(FRAMES) == F


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


def frame(tilt, swing, tongue, bob, q):
    s = [f'<g transform="translate(0 {bob}) rotate({tilt} 50 60)">']
    s.append(ears(swing))
    s.append(f'<ellipse cx="50" cy="54" rx="31" ry="30" fill="{FUR}" stroke="{INK}" stroke-width="4.5"/>')
    s.append(f'<ellipse cx="63" cy="45" rx="10" ry="9" fill="{PATCH}"/>')
    s.append(f'<ellipse cx="50" cy="68" rx="18" ry="13" fill="{MUZZLE}"/>')
    for x in (38, 63):
        s.append(f'<circle cx="{x}" cy="46" r="5.2" fill="{INK}"/><circle cx="{x + 1.7}" cy="44.3" r="1.9" fill="#fff"/>')
    s.append(f'<path d="M45 70H55V{70 + tongue}A5 5 0 0 1 45 {70 + tongue}Z" fill="{TONGUE}" stroke="{INK}" stroke-width="2.5" stroke-linejoin="round"/>'
             f'<path d="M50 72V{68 + tongue}" stroke="#d94a6a" stroke-width="1.5" stroke-linecap="round"/>')
    s.append(f'<path d="M50 63V68M50 68Q44 74 39 69M50 68Q56 74 61 69" fill="none" stroke="{INK}" stroke-width="2.5" stroke-linecap="round"/>')
    s.append(f'<ellipse cx="50" cy="59" rx="7.5" ry="5.5" fill="{INK}"/><ellipse cx="48" cy="57.4" rx="2.6" ry="1.5" fill="#fff" opacity="0.8"/>')
    s.append("</g>")
    s.append(question(q))
    return "".join(s)


out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {F*100} 100" width="{F*100}" height="100">']
for i, f in enumerate(FRAMES):
    out.append(f'<g transform="translate({i*100} 0)">{frame(*f)}</g>')
out.append("</svg>")
sys.stdout.write("\n".join(out) + "\n")
