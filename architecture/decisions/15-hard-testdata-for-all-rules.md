# 15. New test data that is hard even for the new rules

- **Status:** done
- **Date:** 2026-11-07 (`f0d5542`)

Ten puzzles in `testdata/hard` that need lookahead even with all rules.
Searched with the current generator (default options like `generate`), hits
per searched seeds from 1 on:

- N = 6 to 9: 0 of 400 each. N = 10: 3/400, N = 11: 4/400, N = 12: 3/400,
  N = 13: 2/250, N = 14: 4/120. Guessing was never needed.
- Taken over only hits with at least 2 lookahead steps: `n10_s396` (9),
  `n11_s251` (8), `n11_s348` (8), `n12_s082` (2), `n12_s194` (17),
  `n13_s146` (2), `n13_s181` (4), `n14_s009` (25), `n14_s021` (9),
  `n14_s025` (13). Discarded (1 step): N = 10 seeds 22, 225; N = 11 seeds
  142, 248; N = 12 seed 379; N = 14 seed 19.
- Now 14 of 45 puzzles need lookahead with all rules. Because of this,
  `TestPriorityOrder` sees clearly more level 6 for the first time
  (`DeadCells` 40 instead of 2 steps), still 0 violations.
- Cost: `observers` tests 30 → 58 s, `web` tests 5 → 16 s.

The search continued in [17](17-library-over-100-sample-tests.md).
