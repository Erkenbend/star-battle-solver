# 36. Lookahead stops trials that can no longer win

- **Status:** done
- **Date:** 2026-11-22 (`d539e81`)

Suggestion 14 of [refactorings.md](../refactorings.md). `lookahead` ran
every trial to its end and then took the refutation with the fewest steps
(then fewest cells, then the earliest candidate). A trial whose chain is
already longer than a refutation another trial found can never be chosen.

The workers now share `best`, an `atomic.Int32` with the length of the
shortest refutation so far, lowered with a compare-and-swap loop as in the
generator ([decision 13](13-parallel-generator-attempts.md)). Each trial
runs `runSync`, `RunSync` with a step limit: after every new step it reads
the limit and gives up (ending `Stuck`) once the chain is longer than
`best`. `RunSync` is `runSync` without a limit.

- **The choice does not depend on scheduling.** `best` is always the length
  of a real refutation, so it never drops below the shortest chain of all,
  m. A trial is stopped only when its chain is longer than `best` ≥ m, so
  no chain of length m is stopped, and the choice among them (fewest cells,
  then the earliest candidate) is made on the same chains as before. Which
  of the longer trials are cut short, and how early, depends on which
  workers finish first; only the amount of work does.
- **Stopped only when longer, not when equal.** A chain as long as `best`
  can still win with fewer cells or an earlier index, and the cells are
  known only at its end. `TestRunSyncStepLimit` checks that boundary: a run
  exactly as long as the limit ends as with `RunSync`, one step longer is
  given up right after that step (the test fails with `>=` instead of `>`).
- **`atomic.Int32`, not `Int64`.** With `Int64` and `math.MaxInt64` as "none
  yet", `int(best - 1)` wraps on a 32-bit build, and `deploy.sh` builds
  `GOARCH=arm` for a 32-bit Raspberry Pi OS: every trial would be stopped at
  once. Chains are far shorter than 2³¹ steps.
- **Not done: start the candidates in a different order, or skip trials.**
  The candidates already come tightest unit first, which tends to find
  short refutations early. Nothing can be skipped before a trial starts:
  its first step is the assumed star, which may already be a refutation of
  length 1.
- **Same output.** `solve -runner sync -verbose -nocolor` on all 193 puzzles
  of `testdata/hard` (on 16 and on 4 threads), on 120 generated puzzles
  (sizes 8, 10, 12 to 15, seeds 1 to 20) and `hardsearch -n 15 -from 1
  -count 500` (without its times) are byte for byte the same as before;
  `TestTraceGolden` passes unchanged.

Measurements (Ryzen 7 5800X3D, 16 threads, one run each unless noted):

| | before | after | |
|---|---|---|---|
| `solve -runner sync -verbose`, all 193 of `testdata/hard`, sum of wall times | 99.1 s | 66.9 s | 1.48× |
| same, CPU (user + system) | 433 s | 262 s | 1.65× |
| same, the 125 puzzles of N = 13 to 15 with `GOMAXPROCS=4` (as on a Pi 4), wall | 98.3 s | 66.9 s | 1.47× |
| `go test -short -run TestHardPuzzles ./internal/solver/observers`, wall (two runs) | 6.0 s | 5.2 s | 1.15× |
| same, CPU | 23.1 s | 18.9 s | 1.22× |
| `hardsearch -n 15 -from 1 -count 500`, wall (13 hits either way) | 9.5 s | 8.5 s | 1.12× |
| `solve` on the 120 generated puzzles, wall | 3.8 s | 3.6 s | |

By size, `testdata/hard` (sum of wall times, slowest puzzle):

| N | puzzles | before | after | | slowest |
|---|---|---|---|---|---|
| 5 to 9 | 15 | < 0.1 s | < 0.1 s | | |
| 10 | 12 | 1.4 s | 1.1 s | 1.3× | 0.3 → 0.2 s |
| 11 | 21 | 3.6 s | 2.3 s | 1.6× | 1.8 → 0.9 s |
| 12 | 20 | 3.0 s | 2.0 s | 1.5× | 0.7 → 0.4 s |
| 13 | 54 | 31.0 s | 21.2 s | 1.5× | 3.2 → 2.4 s |
| 14 | 53 | 41.3 s | 27.5 s | 1.5× | 5.3 → 3.0 s |
| 15 | 18 | 18.8 s | 12.9 s | 1.5× | 3.9 → 2.3 s |

Per puzzle (the 91 that take over 0.3 s) the ratio is 1.0× to 3.0×,
median 1.46×. The gain is in the puzzles that need lookahead, so the test
sample with `-short` (without the 3-star puzzles) and `hardsearch` (where
generating and the base rules take most of the time) gain less, and
generated puzzles, which mostly need no lookahead, nothing measurable.
The "before" here is the state after decision 35, which already made
the trials cheaper by building explanations only when read.

Open: not measured on the Raspberry Pi itself.
