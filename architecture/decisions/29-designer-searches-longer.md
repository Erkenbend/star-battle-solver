# 29. The designer searches until a budget and keeps the hardest

**Status:** done
**Date:** 2026-11-20

## What changed

Before, "Vorschläge berechnen" ran *k* completions (1 to 8, default 4), one
goroutine each with its own seed and up to 300 attempts, rated each with the
full solver (rules, lookahead, guessing) and showed all of them. With the
bitmask brute force (`91355ac`), a completion takes milliseconds at 8×8 to
about a second at 14×14, so *k* puzzles were a small sample of what the
generator can offer in the same time, and most cores sat idle at the
default *k* = 4.

Now:

- **Workers until a budget.** One worker per core but one loops: fresh seed,
  `generator.Complete` (`Workers: 1`, `MaxAttempts` still 300 per seed),
  rate, repeat. The search ends after `MaxTries` = 100,000 generator
  attempts in total, or when the page stops it ("Anhalten", closing the
  page). A new `generator.Options.Tried` hook counts attempts; it does not
  change any result, so seeds and saved files reproduce as before.
- **The dropdown is an upper limit.** "Höchstens *k* Vorschläge" (1, 2, 4,
  8, 12, 16, 24; default 8) is how many the page shows. The server keeps the
  *k* hardest distinct puzzles and streams each as soon as it is among them.
- **Counters.** Every 500 ms the page gets tried / accepted (uniquely
  solvable and new) / duplicates, shown next to the time.
- **Brute force works, the rules only rate.** Uniqueness is decided by the
  brute-force solver inside the generator, as before. The rule-based solver
  rates every new puzzle with the rules alone (`-search none`); lookahead
  and guessing run only if the rules get stuck *and* the puzzle can still
  enter the *k* hardest.
- **Order.** Harder = more free cells where the rules alone get stuck,
  then more lookahead, then the one that entered the *k* hardest first. The
  last tie-break used to be the seed; with a long search that let every
  puzzle the rules solve compete on a random number, so the page kept
  changing for nothing and each such puzzle needed rating. Now an equally
  hard puzzle never pushes out an earlier one. Go and JS sort the same way,
  so the page's "sort, keep *k*" ends with the server's set whatever order
  candidates arrive in (`TestSearchPageAgrees`).
- **`design` sets `GOGC` to 400** (`debug.SetGCPercent`). With 15 workers
  allocating at once, garbage collection was the largest single cost.

## Measurements

16 cores, 15 workers, *k* = 8, 15 s per pattern (or until the budget):
attempts per second, puzzles accepted, and the CPU time of rating (rules +
search) out of the about 225 CPU-s the workers have.

| pattern | GOGC 100: attempts/s | GOGC 400: attempts/s | accepted (GOGC 400) | rules per puzzle | searches | rating share |
|---|---|---|---|---|---|---|
| `n08_sad_smiley` | 5,695 | 11,855 (budget after 8.4 s) | 21,783 | 2.3 ms | 0 | 40 % |
| `n10_confused_smiley` | 2,065 | 4,472 | 2,006 | 6.0 ms | 15 × 0.34 s | 8 % |
| `n12_inseln` | 1,056 | 2,268 | 414 | 7.0 ms | 1 × 0.39 s | 1.5 % |
| `n14_stairs` | 725 | 1,323 | 55 | 46 ms | 4 × 2.7 s | 5 % |
| empty 10×10 | 773 | 1,614 | 17,928 | 6.6 ms | 37 × 0.38 s | 59 % |
| empty 14×14 | 80 | 141 | 1,585 | 28 ms | 21 × 3.5 s | 41 % |

So the budget lasts about 10 s at 8×8, 20 to 80 s for drawn patterns of
10×10 to 14×14 and about 12 minutes for an empty 14×14. On drawn patterns
the generator gets more than 90 % of the CPU. Rating takes a large share
only where completing is nearly free (small or empty boards), and those
use up the budget quickly. Under full load everything is slower than alone:
the rules take 0.8 ms per 8×8 puzzle alone, 5.5 ms with GOGC 100 and
2.3 ms with GOGC 400 next to 14 other workers; the search at 14×14, whose
lookahead starts its own pool, 0.2 s alone and about 3 s here.

Without the gate, every stuck puzzle would get the search. In a sample of
150 completions of the empty 14×14, the rules got stuck on 5 (3 %); for the
1,585 accepted above that is an estimated 50 searches instead of 21, about
100 CPU-s more in 19 s.

## Rejected

- **Base rules as a cheaper filter before all rules.** Fewer rules give an
  upper bound on the free cells, which could rule a puzzle out early. But
  the base rules (`observers.BaseConfig`) cost about as much as all rules
  (8×8: 0.87 vs. 0.82 ms, 14×14: 4.8 vs. 7.7 ms), because the sync runner
  rarely gets to the look-ahead rules while the base ones keep finding
  something. Not worth a third solver function.
- **A time limit instead of an attempt budget.** The user asked for a
  higher limit of tries; attempts also make runs comparable across
  machines. The page shows the time and can stop at any moment.
- **Raising only the 300 attempts per seed.** They decide when drawn regions
  may grow (`generator.Complete`'s second round) and are part of what a
  saved seed means; the budget is the limit of the search instead.

Open: the hardest completions of a drawing often have its drawn regions
grown noticeably more than easy ones (seen with the heart of the tests at
8×8). Ranking could prefer shapes that stay closer to the drawing.
