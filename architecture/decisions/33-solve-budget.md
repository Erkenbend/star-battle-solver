# 33. One solve budget in `serve`: limit, cancel, cache

- **Status:** partly done (cancel and limit; the cache is open)
- **Date:** 2026-11-22

Suggestion 6 of [refactorings.md](../refactorings.md). Every solve of
`serve` ran the full solver with lookahead (`GOMAXPROCS` workers each) with
no limit across the process and no way to stop it: a crawler opening the
library started one solve per page at once, a visitor who left a page
still had its puzzle solved to the end, and every reload of `/play` or of
an older `/?n=…&seed=…` solved again.

## What was done

1. **`solver.Options.Done`**, like `generator.Options.Done`: checked before
   each lookahead trial and each guess. A cancelled solve ends with the new
   status `Cancelled` (`Err` is `solver.ErrCancelled`) and returns only
   what it had proven: a guess cancelled deep down is neither taken for a
   refuted one (that would cross the cell of the solution's star, or end in
   a false contradiction) nor kept on the board. `TestSolveCancelled`
   cancels at counted calls of the rules, not at times, and failed with
   the propagation of a cancelled guess removed. `web.SolveFunc` takes a
   `context.Context`; pages pass the request's, so does the generator of
   `/?n=…&seed=…` (its `Done` existed already).
2. **`web.Budget`**, sized like `generator.cpuSlots` (GOMAXPROCS − 1):
   every solve of `serve` takes a slot (a buffered channel as semaphore,
   acquired in a `select` with the caller's context). Solves nobody waits
   for yet (the pool, a party's next puzzles, hard library puzzles, whose
   page comes without the result) take a slot of a second semaphore with
   one fewer first, so a page always finds a slot. Nothing holds a slot of
   the budget and of `cpuSlots` at once: callers generate first, then
   solve. [concurrency.md](../concurrency.md), section 11.

## Measurements

`serve -no-pregen` pinned to 4 cores (`taskset -c 0-3`, `GOMAXPROCS=4`,
like a Raspberry Pi 4, but each core several times faster) on a Ryzen 7
5800X3D; a load generator in Go on the other cores, a fresh server per
scenario, one run each. "CPU" is the server's user + system time.

**Crawler:** opens the 193 hard puzzles of the library one after another
as fast as the pages come, then waits for every solve (`/bibliothek/weg`);
meanwhile a player asks for a new 10×10 puzzle (not pooled) every 300 ms,
and after the crawl three motif pages are opened (they wait for their
solve).

| | before | one semaphore | budget (two lanes) |
|---|---|---|---|
| crawler page, slowest | 3.10 s | 0.01 s | 0.01 s |
| motif page during the crawl, slowest of 3 | 1.50 s | **53.7 s** | 0.00 s |
| new 10×10 during the crawl, median / slowest | 0.16 / 3.14 s | 0.02 / **42.6 s** | 0.01 / 0.05 s |
| all library solves done | 62.1 s | 56.2 s | 58.1 s |
| server CPU | 246 s | 219 s | 220 s |
| peak RSS | 369 MB | 161 MB | 163 MB |

Before, 193 solves ran at once: the four cores were saturated for a minute
and every other request waited behind them. With one semaphore, as the
suggestion had it, the work is done sooner and with less memory (less
contention and garbage), but a solve a page waits for queues behind the
whole crawl, first in, first out. Hence the second lane: background solves
take at most two of the three slots.

**Abandoned pages:** 12 visitors paste different hard 14×14 and 15×15
puzzles into `/play` at once and leave after 300 ms.

| | before | budget |
|---|---|---|
| server CPU in the 16 s after they left | 12.5 s | 0.13 s |
| peak RSS | 55 MB | 32 MB |

**Flood:** 12 visitors paste different hard puzzles at once and stay.

| | before | budget |
|---|---|---|
| latency, median / slowest | 2.68 / 3.69 s | 2.11 / 4.18 s |
| all done | 3.7 s | 4.2 s |
| peak RSS | 54 MB | 32 MB |

The cost of the limit: three solves on four cores leave a core idle while
the solves are in their rules phase (single-threaded), so a burst of pages
that all wait takes a little longer overall, about 13 % here.

`TestHardPuzzles` takes 6.2 s with and without step 1, and
`TestTraceGolden` shows the same traces: a `Done` that stays open changes
nothing.

## Alternatives

- **One semaphore**, as suggested: measured above, it starves the pages
  during a crawl.
- **Priorities in one semaphore** (a mutex and two queues): the same
  effect as the two lanes in more code; two buffered channels are the
  idiom the generator already uses.
- **Cancelling the library's solves** when their visitor leaves: a solve
  there is shared by everyone who opens the puzzle and its result kept, so
  it runs with `context.Background()` and only waits in the budget.
- **Making the lookahead's workers take `cpuSlots` tokens**, one budget for
  every core of the process: stricter, but a single solve would then wait
  for the generator and lose its parallelism; not needed by the numbers.

Open: step 3 of the suggestion, a bounded cache of served results and the
tutorial's result computed once.
