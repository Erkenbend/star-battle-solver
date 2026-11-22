# 32. Precomputed geometry for `board`

- **Status:** done
- **Date:** 2026-11-22 (`202567b`)

Suggestion 5 of [refactorings.md](../refactorings.md). A CPU profile of the
hard puzzle test showed `board.Implied` (a map and `RegionCells` rebuilt for
every star `solver.Apply` sets, also in every `assume` of the rules), `Free`
building slices that were only counted and `Contradiction` rebuilding all 3N
units after every change.

`board.New` now builds a `geometry` per state: the units (as
`puzzle.Units`), the implied crosses of every cell (in the order `Implied`
always had) and a table "a star on cell crosses other". `Clone` shares it
as a pointer. `State.Implied`, `State.Units` and `State.CrossesAll` read
from it; `Contradiction` walks its units; `Elimination` lost its own
`impliedTable`. `FreeCount` and `FirstFree` replace `Free` where only a
count or the first cell is needed.

- **Built eagerly in `New`, not cached lazily on the puzzle.** Many
  goroutines share one `*puzzle.Puzzle`, so a lazy cache would need a
  `sync.Once` or a mutex. Built in `New`, it is written before any goroutine
  gets a clone and only read afterwards
  ([concurrency.md](../concurrency.md), intro and section 1). Cost: one
  build per `New` (for N = 15, 225 lists of up to 56 cells and 50,625
  bools), which is small next to one solve. `New` runs once per solve, not
  per trial.
- **`Open(unit)` was not added:** the callers either need `HasStar` alone
  or `FreeCount == 0 && !HasStar`, which reads well as it is.
- **Same output.** `solve -runner sync -verbose -nocolor` on all 193
  puzzles of `testdata/hard`, `html` on the 29 sample puzzles and `solve`
  on 120 generated puzzles (sizes 8, 10, 12 to 15, seeds 1 to 20) are byte
  for byte the same as before.

Measurements (Ryzen 7 5800X3D, 16 threads, one run each):

| | before | after | |
|---|---|---|---|
| `solve -runner sync -verbose`, all 193 of `testdata/hard`, sum of wall times | 255 s | 104 s | 2.4× |
| same, CPU (user + system) | 1085 s | 481 s | 2.3× |
| `go test -short -run TestHardPuzzles ./internal/solver/observers`, wall | 13.2 s | 5.4 s | 2.4× |
| same, CPU | 51.8 s | 22.4 s | 2.3× |
| `observers.assume` in that test's profile (cumulative) | 12.6 s | 1.8 s | |
| `runtime.mallocgc` in that profile (cumulative) | 12.6 s | 4.5 s | |

By size, `testdata/hard` (sum of wall times, slowest puzzle):

| N | puzzles | before | after | | slowest |
|---|---|---|---|---|---|
| 5 to 9 | 15 | 0.1 s | 0.1 s | ~1.1× | |
| 10 | 12 | 3.5 s | 1.6 s | 2.2× | 0.7 → 0.3 s |
| 11 | 21 | 11.3 s | 3.9 s | 2.9× | 5.4 → 1.8 s |
| 12 | 20 | 8.4 s | 3.4 s | 2.5× | 2.0 → 0.8 s |
| 13 | 54 | 78.2 s | 33.0 s | 2.4× | 8.5 → 3.4 s |
| 14 | 53 | 104.3 s | 42.7 s | 2.4× | 13.2 → 5.5 s |
| 15 | 18 | 49.2 s | 19.8 s | 2.5× | 10.3 → 4.1 s |

Per puzzle the ratio is 1.0× to 5.4×, median 2.4×. Generated puzzles mostly
solve in milliseconds, where process start dominates (1.1× to 1.4× per
size); the 20 puzzles of size 14, two of which need search, went from
2.18 s to 0.86 s (2.5×). The generator itself does not use `board` and is
unchanged.

Open: not measured on the Raspberry Pi. In the new profile the largest
remaining parts are `Confinement` (`confinedSets`, 25 % cumulative) and
`openUnits` (16 %, called by `Pigeonhole`), both mostly map writes and
iteration; GC is still about 15 %.
