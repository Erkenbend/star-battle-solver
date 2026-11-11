# Writes the animated sprite sheet symbols/sparkle.svg to stdout: 8 frames of
# a turning, pulsing star with a sparkle, side by side, 100×100 each.
import math, sys

F = 8
pts = [(50, 6), (61.5, 37.2), (94.7, 38.5), (68.5, 59), (77.6, 91), (50, 72.5), (22.4, 91), (31.5, 59), (5.3, 38.5), (38.5, 37.2)]
poly = " ".join(f"{x},{y}" for x, y in pts)
out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {F*100} 100" width="{F*100}" height="100">']
for i in range(F):
    t = i / F
    rot = 72 * t  # 5-fold symmetry: loops after 72°
    s = 0.9 + 0.08 * math.sin(2 * math.pi * t)
    a = max(0.0, math.sin(2 * math.pi * t))  # the sparkle shows half the time
    out.append(f'<g transform="translate({i*100} 0)">')
    out.append(
        f'<g transform="translate(50 52) rotate({rot:.1f}) scale({s:.3f}) translate(-50 -52)">'
        f'<polygon points="{poly}" fill="#f5b800" stroke="#5c3d00" stroke-width="5" stroke-linejoin="round"/>'
        f'<polygon points="50,22 55,40 50,50 45,40" fill="#fff6c8" opacity="0.75"/></g>'
    )
    if a > 0.01:
        out.append(
            f'<path transform="translate(83 17) scale({0.6 + 0.6 * a:.3f})" d="M0 -14L3 -3 14 0 3 3 0 14-3 3-14 0-3-3Z" '
            f'fill="#ffffff" stroke="#5c3d00" stroke-width="1.5" opacity="{a:.2f}"/>'
        )
    out.append("</g>")
out.append("</svg>")
sys.stdout.write("\n".join(out) + "\n")
