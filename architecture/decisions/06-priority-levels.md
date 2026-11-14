# 6. Priority levels for the observers

- **Status:** done
- **Date:** 2026-11-05 (`4a67099`)

Every rule reports its level through `Priority()`, and both runners apply a
rule only when no simpler one finds anything. The levels and how each runner
honours them are described in `PLAN.md`, section "Regeln".

Measurement over 80 puzzles (the 32 hard ones + 48 generated with N = 5 to
10), rules only, steps per level L1 to L6:

- `RunSync`: 657 / 233 / 323 / 63 / 118 / 2, 0 violations.
- `RunConcurrent` before: 657 / 122 / 475 / 353 / 656 / 642, **2013
  violations**, i.e. `ForcedPartner` and `DeadCells` often fired although a
  simple rule applied.
- `RunConcurrent` after: 657 / 229 / 327 / 61 / 112 / 2, 0 violations. Cost:
  `TestHardPuzzles` 3.2 s instead of 2.0 s.

Open (small): within one level, the concurrent runner takes the waiting
message with the smallest index and does not wait for slower observers of the
same level. If needed, finer levels (e.g. one per k for `Confinement`).
