# 12. Minimum region size 2 from N = 10 on

- **Status:** done
- **Date:** 2026-11-06 (`48ea102`)

`generator.DefaultMinRegionSize(n)`, used by `generate` (override with
`-min-region K`) and `serve`. From N = 10 on, the same seeds therefore give
different puzzles than before.

Measurement with 50 puzzles each for N = 12 to 15, minimum size 1 → 2:

- **Generating:** hardly any more discarded layouts (median 1 to 2 attempts,
  max. 5), but the repair takes longer: median 0.07 → 0.13 s (N = 12),
  0.23 → 0.56 s (13), 1.1 → 2.8 s (14), 10 → 22 s (15); 90 % quantile at
  N = 15 41 → 100 s. So N = 15 is too slow for `serve`, whatever the minimum
  size. (These times ran with 14 puzzles at once and are too high; alone, N =
  15 takes about 5 s in the median instead of 22 s, see
  [13](13-parallel-generator-attempts.md).)
- **Regions:** with minimum size 1, 47 to 49 of 50 puzzles have a one-cell
  region (a free star). With 2, on average about 2 regions of exactly 2 cells
  remain (before: 2.4 to 3 regions with ≤ 2 cells): the repair shrinks
  regions down to the limit, it does not make them more balanced.
- **Difficulty:** solving paths 20 to 30 % longer (avg. steps 19.9 → 24.3 /
  20.9 → 27.4 / 22.2 → 28.6 / 25.3 → 30.4). Steps of level 5 or above per
  puzzle 0.44 → 0.72 / 0.10 → 1.10 / 0.16 → 0.92 / 0.64 → 0.98. Puzzles whose
  hardest step is at most level 2: 7 to 9 → 1 to 3 of 50. Base rules without
  lookahead: 45 → 46, 48 → 43, 47 → 41, 45 → 43 of 50. All rules: at N = 14,
  3 of 50 need lookahead (→ new test data `n14_s002/024/029`), otherwise 0 or
  1 of 50; too few cases to conclude anything.
- Cost of the new test data: `observers` tests 19 → 29 s, `web` tests
  1.3 → 5.5 s.
