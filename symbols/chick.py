# Writes the animated sprite sheet symbols/chick.svg to stdout (see sheet.py):
# a chick peeking out of its egg that peeps, wobbles the egg, or hops out
# flapping so the shell cap flies up.
from sheet import sheet

INK = "#5c3d00"
SHELL = "#fdf6e3"
SPOT = "#e8d9b5"
CHICK = "#ffd84a"
BEAK = "#ff9a2e"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Peeps twice, the cap bobbing.
    "peep": [dict(beak=1, cap=4, notes=0.6), dict(beak=1, cap=6, notes=1), {}, dict(beak=1, cap=4, notes=0.6),
             dict(beak=1, cap=6, notes=1), {}, {}, {}],
    # Wobbles the egg from side to side.
    "wobble": [dict(rock=-10), dict(rock=-14, eyes="shut"), dict(rock=-6), dict(rock=6), dict(rock=14, eyes="shut"),
               dict(rock=10), dict(rock=0), dict(rock=-6), dict(rock=4), {}],
    # Hops out flapping, the cap flies up and lands back on the head.
    "hatch": [dict(rise=4, cap=4), dict(rise=12, cap=14, wings=1, eyes="happy"), dict(rise=16, cap=26, wings=-1, eyes="happy", beak=1),
              dict(rise=16, cap=34, wings=1, eyes="happy", beak=1), dict(rise=14, cap=30, wings=-1, eyes="happy"),
              dict(rise=8, cap=16, wings=1), dict(rise=2, cap=4), {}, {}],
}

EGG = "M19 60L27 67 35 59 43 67 51 59 59 67 67 59 75 67 81 60C83 81 70 95 50 95C30 95 17 81 19 60Z"
CAP = "M30 38L35 44 41 37 47 44 53 37 59 44 65 37 70 38C70 25 61 16 50 16C39 16 30 25 30 38Z"


def frame(beak=0, cap=0, notes=0, rock=0, eyes="open", rise=0, wings=0):
    rise += 5  # at rest the chick peeks out just above the rim
    s = [f'<g transform="rotate({rock} 50 95)">']
    # The chick, below the egg's rim.
    s.append(f'<g transform="translate(0 {-rise})">')
    if wings:
        a = -55 if wings > 0 else 25
        for side in (-1, 1):
            x = 50 + side * 21
            s.append(f'<ellipse cx="{x}" cy="62" rx="11" ry="6.5" fill="{CHICK}" stroke="{INK}" stroke-width="3.5" '
                     f'transform="rotate({side * a} {50 + side * 14} 62)"/>')
    s.append(f'<circle cx="50" cy="55" r="23" fill="{CHICK}" stroke="{INK}" stroke-width="4.5"/>')
    for x in (41, 59):
        if eyes == "open":
            s.append(f'<circle cx="{x}" cy="52" r="3.6" fill="{INK}"/><circle cx="{x + 1.2}" cy="50.8" r="1.2" fill="#fff"/>')
        elif eyes == "happy":
            s.append(f'<path d="M{x - 4} 54Q{x} 47 {x + 4} 54" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
        else:
            s.append(f'<path d="M{x - 4} 52Q{x} 56 {x + 4} 52" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    s.append('<ellipse cx="34" cy="60" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.55"/>'
             '<ellipse cx="66" cy="60" rx="4.5" ry="2.8" fill="#ff8a7a" opacity="0.55"/>')
    if beak:
        s.append(f'<path d="M45 57L50 53 55 57Z" fill="{BEAK}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>'
                 f'<path d="M46 60L50 65 54 60Z" fill="{BEAK}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>')
    else:
        s.append(f'<path d="M45 57L50 54 55 57 50 61Z" fill="{BEAK}" stroke="{INK}" stroke-width="2" stroke-linejoin="round"/>')
    s.append("</g>")
    s.append(f'<path d="{EGG}" fill="{SHELL}" stroke="{INK}" stroke-width="4.5" stroke-linejoin="round"/>')
    s.append(f'<ellipse cx="36" cy="80" rx="4" ry="3" fill="{SPOT}"/><ellipse cx="62" cy="84" rx="5" ry="3.5" fill="{SPOT}"/>'
             f'<ellipse cx="55" cy="73" rx="3" ry="2.2" fill="{SPOT}"/>')
    s.append(f'<g transform="translate(0 {-rise - cap}) rotate({cap * 0.6} 50 30)">'
             f'<path d="{CAP}" fill="{SHELL}" stroke="{INK}" stroke-width="4" stroke-linejoin="round"/>'
             f'<ellipse cx="44" cy="27" rx="3" ry="2" fill="{SPOT}"/></g>')
    s.append("</g>")
    if notes:
        # Two little notes for the peep.
        s.append(f'<g opacity="{notes}" fill="{INK}" stroke="{INK}" stroke-width="2.4" stroke-linecap="round">'
                 f'<path d="M84 26V12L92 10V22" fill="none"/><ellipse cx="82" cy="27" rx="3.2" ry="2.4"/><ellipse cx="90" cy="23" rx="3.2" ry="2.4"/>'
                 f'<path d="M16 22V10" fill="none"/><ellipse cx="14" cy="23" rx="3.2" ry="2.4"/></g>')
    return "".join(s)


sheet(frame, REST, CLIPS)
