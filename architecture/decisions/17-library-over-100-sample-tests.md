# 17. Library over 100 puzzles, tests only with a sample

- **Status:** done; CI running all puzzles replaced by [22](22-ci-sample-only.md)
- **Date:** 2026-11-12 (`13ff7e6`, `05879ce` to `dabdb5e`, `93c508e`)

73 new puzzles in `testdata/hard` (now 118); all of them need lookahead even
with all rules. Local tests check only the 9 puzzles from `sample.idx`;
`STARBATTLE_ALL_HARD=1` and CI check all of them (see `PLAN.md`,
"Verifikation").

- Searched as last time ([15](15-hard-testdata-for-all-rules.md)),
  continuing from the seeds of the last search. Hits (lookahead with all
  rules) / of those taken over (at least 4 lookahead steps) / searched seeds:
  N = 10: 15 / 9 / 2000 (seeds 401 to 2400), N = 11: 14 / 3 / 2000 (401 to
  2400), N = 12: 13 / 7 / 1600 (401 to 2000), N = 13: 39 / 25 / 2750 (251 to
  3000), N = 14: 36 / 29 / 1680 (121 to 1800). Guessing was never needed.
- The threshold is higher than last time (2 steps), because there were enough
  hits. Longest chains: `n13_s1787` (53 steps), `n14_s1345` (43),
  `n13_s2107` (35), `n11_s851` (30), `n14_s835` (30).
- Slowest to solve: `n14_s1262` (12.6 s) and `n13_s1787` (a good 10 s),
  previously at most 3 s. Correspondingly longer on the Pi; the library starts
  the solver when a puzzle is opened, and whoever solves by hand takes longer
  anyway.

The next search continues at N = 10 to 12 from seeds 2401/2401/2001, N = 13
from 3001, N = 14 from 1801.
