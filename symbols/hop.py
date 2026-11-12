# Writes the animated sprite sheet symbols/hop.svg to stdout: 10 frames of a
# smiling star, side by side, 100×100 each. It crouches, hops up with a
# wink and a little sparkle, and lands with a squash. Frame 0 is the pose
# that stays on the board.
import sys

F = 10
INK = "#5c3d00"
pts = [(50, 6), (61.5, 37.2), (94.7, 38.5), (68.5, 59), (77.6, 91), (50, 72.5), (22.4, 91), (31.5, 59), (5.3, 38.5), (38.5, 37.2)]
poly = " ".join(f"{x},{y}" for x, y in pts)
BASE = 0.86  # star size at rest, so the hop stays inside the box
GROUND = 94

# Per frame: (lift, scale x, scale y, wink, sparkle)
FRAMES = [
    (0, 1.00, 1.00, 0, 0),
    (0, 1.14, 0.84, 0, 0),
    (8, 0.90, 1.10, 0, 0),
    (16, 0.96, 1.04, 1, 0.5),
    (19, 1.00, 1.00, 1, 1),
    (16, 0.98, 1.02, 1, 0.6),
    (7, 0.92, 1.08, 0, 0),
    (0, 1.16, 0.82, 0, 0),
    (0, 0.96, 1.05, 0, 0),
    (0, 1.02, 0.98, 0, 0),
]
assert len(FRAMES) == F


def face(wink):
    s = [f'<ellipse cx="42" cy="49" rx="3.6" ry="5" fill="{INK}"/>']
    if wink:
        s.append(f'<path d="M53 50Q58 44 63 50" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
    else:
        s.append(f'<ellipse cx="58" cy="49" rx="3.6" ry="5" fill="{INK}"/>')
    s.append(f'<path d="M43 58Q50 66 57 58" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
    s.append('<ellipse cx="35" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>'
             '<ellipse cx="65" cy="57" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.6"/>')
    return "".join(s)


def frame(lift, sx, sy, wink, sparkle):
    s = []
    # Squash and stretch about the bottom, then lift.
    s.append(f'<g transform="translate(50 {GROUND - lift}) scale({BASE * sx:.3f} {BASE * sy:.3f}) translate(-50 -91)">')
    s.append(f'<polygon points="{poly}" fill="#f5b800" stroke="{INK}" stroke-width="5" stroke-linejoin="round"/>')
    s.append('<polygon points="50,20 54,36 50,42 46,36" fill="#fff6c8" opacity="0.75"/>')
    s.append(face(wink))
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


out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {F*100} 100" width="{F*100}" height="100">']
for i, f in enumerate(FRAMES):
    out.append(f'<g transform="translate({i*100} 0)">{frame(*f)}</g>')
out.append("</svg>")
sys.stdout.write("\n".join(out) + "\n")
