# 13. Generator: attempts in parallel

- **Status:** done
- **Date:** 2026-11-06 (`7a1d799`, `ff3b10a`)

`Options.Workers` (default 4, at most GOMAXPROCS) attempts run at the same
time, limited by a buffered channel. Each attempt draws its own random
generator from `rng` in order; the successful attempt with the **smallest
index** wins, later ones are no longer started or stop during the repair. So
the result depends only on the seed, not on the number of workers
(`TestWorkersSameResult`); compared to before, though, all seeds give
different puzzles.

- **One core stays free:** across all `Generate` calls of the process (e.g.
  pool and request in `serve`), at most GOMAXPROCS − 1 attempts run at the
  same time (at least 1, `cpuSlots`); the default for `Workers` is
  accordingly min(4, GOMAXPROCS − 1). On the Raspberry Pi (4 cores) that is
  3. The solver (parallel lookahead) is not covered by this.
- Consequence of reproducibility: if attempt 0 succeeds (the majority), it
  still has to be waited for; there is only a gain when it fails. Then the
  next attempts are already running in parallel.
- Measurement (wall time per puzzle, puzzles one after another, seeds 1000
  on, 3:40 min), 1 worker → 4 workers, sum over all puzzles: N = 12 (30
  puzzles, attempt 0 fails for 8) 1.12×; N = 13 (20, 5) 1.10×; N = 14 (17, 7)
  1.32×, maximum 17.7 → 10.9 s; N = 15 (4, 1) 1.23×. Only the puzzles whose
  attempt 0 failed: 1.3× to 1.5× (N = 15: 1.9×, one puzzle).
- 16 workers are worse than 4 (N = 12 to 14: 1.02× / 0.99× / 1.27×): the
  extra attempts slow attempt 0 down by about 14 % (with 4 workers about
  3 %), and hardly any puzzle needs more than 4 attempts.
- Side finding: the times of the measurement for the minimum size
  ([12](12-min-region-size-2.md)) ran with 14 puzzles at once and are too
  high because of it; alone, N = 15 takes about 5 s in the median instead of
  22 s.

Open: a failed attempt runs until it gives up after 6·N repair steps and so
blocks the winner behind it. Giving up earlier (e.g. when the number of
alternatives does not drop) would speed up exactly these cases.
