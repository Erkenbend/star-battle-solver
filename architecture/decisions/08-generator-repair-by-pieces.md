# 8. Generator: repair by moving region pieces

- **Status:** done
- **Date:** 2026-11-05 (`569d98a`)

Goal: fewer discarded layouts. Freshly grown layouts are practically never
unique (median about 124 solutions at N = 8, over 5000 at N = 12); everything
depends on the repair. The old repair mostly failed because the star of the
alternative was a bottleneck of its region (70 to 87 % of the failures), and
it went round in circles (55 to 60 % of the alternatives seen had already
been destroyed once).

- **A piece instead of a cell:** star x of an alternative in region A moves
  into a neighbouring region B, together with everything that would be cut
  off from A's solution star without x (if x lies inside A, additionally the
  shortest path to A's border). A stays connected, the solution stays valid.
- **Scoring over up to 8 alternatives per step:** an alternative dies exactly
  when it has a star in the piece. Points: 10 per destroyed alternative, −20
  per revived, previously destroyed alternative, −1 per moved cell.
- **Give up after 6·N steps** instead of 4·N² (successful repairs need at most
  about 4.5·N).
- `Options.MinRegionSize` (discards puzzles with smaller regions; the repair
  never shrinks a region below it), `Options.Alternatives`.

Result, 40 puzzles each: N = 10 in 0.09 s instead of 1.6 s, N = 12 in 1.8 s
instead of 151 s. With `MinRegionSize` 3 about 3 to 5 times as many attempts.
Logic alone solves about the same share as before (135 to 139 of 140 instead
of 137 to 140).

Counter-check: balanced or compact growing of the regions makes uniqueness
clearly worse (2 to 3 times as many solutions at N = 8). Crooked, unequal
regions make solutions unique. So balanced regions (`PLAN.md`, "Später")
must come from a constraint on the repair, not from growing.
