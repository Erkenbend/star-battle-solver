# 22. CI runs only the sample of hard puzzles

- **Status:** done
- **Date:** 2026-11-16

With 175 puzzles in the library ([21](21-library-175.md)), the CI run with
`STARBATTLE_ALL_HARD=1` took 36 min. Nearly all of it went to the three
packages that go through the hard puzzles: `internal/solver/observers`
2167 s (`TestHardPuzzles`, `TestPriorityOrder`), `internal/web` 1075 s
(`TestJSLogic`) and `internal/hardsearch` 1076 s (`TestLibraryRatings`), 72
min together, run side by side on the runner's cores. CI now runs the
same sample as a plain local `go test ./...`. All puzzles stay one
environment variable away (`STARBATTLE_ALL_HARD=1 go test ./...`), worth
running after a change to the rules, the search or the rating.

- `sample.idx` grows from 9 to 27 puzzles, chosen by size, rule and rating:
  8 × 1 star (N = 5 to 12), 10 × 2 stars, 9 × 3 stars (N = 10 to 14). The
  3-star ones include long chains of thought experiments (chain of 7 to 9:
  `n11_s504`, `n14_s1717`, `n14_s2876`, `n13_s4661`, the deepest of the
  library) and many lookahead steps (25 and 41). The 9 old ones stay, so the
  rules they were chosen for stay covered.
- Cost measured as one sync solve per puzzle (`starbattle solve -runner sync`,
  16 cores): the whole library 202 s (1 star 0.3 s, 2 stars 40 s, 3 stars
  162 s), the old sample 1.4 s, the new one 20 s, about a tenth. Left out are
  the most expensive ones, `n14_s1262` (12.8 s) and `n13_s1787` (8.3 s).
  All three packages solve each puzzle a fixed number of times, so their time
  should shrink with the solving time, to about 220 s, 110 s and 110 s; the
  test step from 36 to about 4 min. Estimated, not measured.
- The cheap 1-star puzzles stay first, because with `-short` `TestJSLogic`
  and `TestLibraryRatings` take only the first few. `TestHardPuzzles` now
  skips the 3-star ones with `-short` (otherwise 48 s instead of 14 s
  locally, and CI runs `-race -short`).
- Alternative: keep all puzzles in CI, but on a schedule (nightly) instead of
  every push. Rejected for now: a failure would show up away from the commit
  that caused it, and the full run is easy to start by hand.
