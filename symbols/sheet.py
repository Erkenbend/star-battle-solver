# Writes an animated sprite sheet for symbols.json: frames of 100×100 side
# by side to stdout, first the resting pose (frame 0, the one that stays on
# the board), then the clips, of which the page plays one at random when a
# star is set. Their "frames" and "clips" for symbols.json go to stderr.
import json, sys


def sheet(frame, rest, clips):
    """frame(**f) draws a frame f, a dict; clips maps names to lists of frames."""
    frames, meta = [rest], []
    for name, fs in clips.items():
        meta.append({"name": name, "start": len(frames), "frames": len(fs)})
        frames += fs
    n = len(frames)
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {n*100} 100" width="{n*100}" height="100">']
    for i, f in enumerate(frames):
        out.append(f'<g transform="translate({i*100} 0)">{frame(**f)}</g>')
    out.append("</svg>")
    sys.stdout.write("\n".join(out) + "\n")
    sys.stderr.write(f'"frames": {n}, "fps": ?, "clips": {json.dumps(meta, ensure_ascii=False)}\n')
