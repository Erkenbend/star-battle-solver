# Writes the animated sprite sheet symbols/ghost.svg to stdout (see sheet.py):
# a little ghost that shouts "boo", fades away and comes back, or floats
# about sticking its tongue out.
from sheet import sheet

INK = "#2c2c4a"
BODY = "#fbfbff"
SHADE = "#dcdcf2"
TONGUE = "#ff6f8e"

# A frame: keyword arguments of frame(); missing ones as at rest.
REST = {}
CLIPS = {
    # Puffs up and shouts "boo", then giggles.
    "boo": [dict(grow=0.04, arms=0.5, mouth="o"), dict(grow=0.1, arms=1, mouth="boo", brows=1), dict(grow=0.12, arms=1, mouth="boo", brows=1),
            dict(grow=0.12, arms=1, mouth="boo", brows=1), dict(grow=0.04, arms=0.3), dict(eyes="happy", mouth="grin", tilt=-6),
            dict(eyes="happy", mouth="grin", tilt=6), dict(eyes="happy", mouth="grin", tilt=-6), {}],
    # Fades away while rising and comes back.
    "fade": [dict(alpha=0.8, lift=2, eyes="shut"), dict(alpha=0.55, lift=4, eyes="shut"), dict(alpha=0.3, lift=6, eyes="shut"),
             dict(alpha=0.12, lift=7, eyes="shut"), dict(alpha=0.12, lift=7, eyes="shut"), dict(alpha=0.35, lift=5),
             dict(alpha=0.65, lift=3, mouth="o"), dict(alpha=1, lift=1, mouth="o"), {}, {}],
    # Floats up and down with a waving hem, tongue out.
    "float": [dict(lift=2, wave=1), dict(lift=4, wave=0, mouth="bleh", eyes="wink"), dict(lift=5, wave=1, mouth="bleh", eyes="wink"),
              dict(lift=4, wave=0, mouth="bleh", eyes="wink"), dict(lift=2, wave=1), dict(lift=0, wave=0), dict(lift=-1, wave=1), {}],
}


def body(wave):
    # The hem waves by moving its points between two phases.
    hem = "L69 82 60 90 50 82 40 90 31 82Z" if not wave else "L70 88 61 82 50 89 39 82 30 88Z"
    return f"M22 90L22 46C22 25 34 12 50 12C66 12 78 25 78 46L78 90{hem}"


def frame(grow=0, arms=0, mouth="small", brows=0, eyes="open", tilt=0, alpha=1, lift=0, wave=0):
    s = [f'<g opacity="{alpha}" transform="translate(0 {-lift}) rotate({tilt} 50 90) translate(50 90) scale({1 + grow}) translate(-50 -90)">']
    for side in (-1, 1):
        # Little arms, raised when spooking.
        a = side * (-10 - 70 * arms)
        s.append(f'<ellipse cx="{50 + side * 30}" cy="60" rx="9" ry="6" fill="{BODY}" stroke="{INK}" stroke-width="4" '
                 f'transform="rotate({a} {50 + side * 24} 60)"/>')
    s.append(f'<path d="{body(wave)}" fill="{BODY}" stroke="{INK}" stroke-width="4.5" stroke-linejoin="round"/>')
    s.append(f'<path d="M66 22C73 28 74 40 74 52" fill="none" stroke="{SHADE}" stroke-width="5" stroke-linecap="round"/>')
    for x in (40, 60):
        if eyes == "happy":
            s.append(f'<path d="M{x - 5} 45Q{x} 38 {x + 5} 45" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
        elif eyes == "shut" or eyes == "wink" and x == 60:
            s.append(f'<path d="M{x - 5} 43Q{x} 48 {x + 5} 43" fill="none" stroke="{INK}" stroke-width="3.2" stroke-linecap="round"/>')
        else:
            s.append(f'<ellipse cx="{x}" cy="43" rx="4" ry="5.5" fill="{INK}"/><circle cx="{x + 1.4}" cy="41" r="1.5" fill="#fff"/>')
    if brows:
        s.append(f'<path d="M33 33 45 37M67 33 55 37" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    s.append('<ellipse cx="32" cy="52" rx="4.5" ry="2.8" fill="#ff9ab0" opacity="0.6"/>'
             '<ellipse cx="68" cy="52" rx="4.5" ry="2.8" fill="#ff9ab0" opacity="0.6"/>')
    if mouth == "boo":
        s.append(f'<ellipse cx="50" cy="62" rx="9" ry="11" fill="#4a2440" stroke="{INK}" stroke-width="3"/>'
                 f'<ellipse cx="50" cy="68" rx="5.5" ry="3.5" fill="{TONGUE}"/>')
    elif mouth == "o":
        s.append(f'<ellipse cx="50" cy="56" rx="3.5" ry="4.5" fill="#4a2440" stroke="{INK}" stroke-width="2"/>')
    elif mouth == "grin":
        s.append(f'<path d="M42 54Q50 64 58 54Z" fill="#4a2440" stroke="{INK}" stroke-width="2.5" stroke-linejoin="round"/>')
    elif mouth == "bleh":
        s.append(f'<path d="M44 55Q50 58 56 55" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>'
                 f'<path d="M47 56H55V61A4 4 0 0 1 47 61Z" fill="{TONGUE}" stroke="{INK}" stroke-width="2.2" stroke-linejoin="round"/>')
    else:
        s.append(f'<path d="M46 55Q50 59 54 55" fill="none" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    s.append("</g>")
    if mouth == "boo":
        s.append(f'<path d="M84 18 94 12M86 28 98 28M14 18 4 12M12 28 0 28" stroke="{INK}" stroke-width="3" stroke-linecap="round"/>')
    return "".join(s)


sheet(frame, REST, CLIPS)
