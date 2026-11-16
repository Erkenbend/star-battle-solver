# 21. Library with 175 puzzles

- **Status:** done
- **Date:** 2026-11-16

57 new puzzles in `testdata/hard` (now 175, 144 of them need lookahead even
with all rules). The search ran with `starbattle hardsearch` at commit
`ffc2880` ([18](18-hard-command.md)) and continued from the seeds of the
last search ([17](17-library-over-100-sample-tests.md)). The generator is
unchanged since `e76b5ce` (only its tests changed), so the same seeds give the
same puzzles as then.

- Same threshold as last time: at least 4 lookahead steps with all rules.
  Hits (lookahead with all rules) / of those taken over (at least 4 steps) /
  searched seeds:
  N = 10: 9 / 0 / 2000 (seeds 2401 to 4400),
  N = 11: 14 / 7 / 2000 (2401 to 4400),
  N = 12: 16 / 5 / 2000 (2001 to 4000),
  N = 13: 50 / 27 / 2000 (3001 to 5000),
  N = 14: 27 / 18 / 1500 (1801 to 3300). Guessing was never needed.
- Ratings ([20](20-difficulty-rating.md)) of the new ones: 27 × 2 stars,
  30 × 3 stars, none with 1 star (at least 4 lookahead steps always mean 2
  stars or more). The library now has 31 / 73 / 71 puzzles with 1 / 2 / 3
  stars.
- Longest chains: `n13_s4661` (41 steps), `n13_s4338` (34), `n14_s2329` (31),
  `n14_s3133` (26), `n14_s2876` (25).
- Solving time, one puzzle at a time (`starbattle solve`, 16 cores): at most
  6.4 s (`n13_s4661`), then `n14_s3146` and `n14_s2329` (5.5 and 5.4 s); all
  57 together 108 s. The slowest of the library stays `n14_s1262` (13.3 s).
  The times the search prints are much higher (up to 47 s), because it solves
  16 seeds at once.
- Cost for the tests: solving the old 118 one by one takes 162 s, the new
  ones add 108 s (×1.7). The full run with `STARBATTLE_ALL_HARD=1` (and so
  CI) should go from about six to about ten minutes; estimated, not measured
  locally. The normal run is unchanged, because `sample.idx` stays the same.
- N = 10 gave nothing in 2000 seeds: the 9 hits there needed only 1 or 2
  lookahead steps. Small sizes are better left out of the next search, or
  searched with a lower threshold.

The next search continues at N = 10 and 11 from seed 4401, N = 12 from 4001,
N = 13 from 5001, N = 14 from 3301.
