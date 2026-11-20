# 29. The designer searches on all cores until it has k puzzles

**Status:** done
**Date:** 2026-11-20

## What changed

Before, "Vorschläge berechnen" ran *k* completions (1 to 8, default 4), one
goroutine each with its own seed and up to 300 attempts. A seed that found
nothing in its 300 attempts was a failed proposal, so patterns the generator
rarely completes gave few or none, and at the default *k* = 4 most cores sat
idle. With the bitmask brute force (`91355ac`), an attempt costs well under
a millisecond at 8×8 and a few at 14×14, so a much larger budget is
affordable.

Now:

- **Workers until k puzzles.** One worker per core but one loops: fresh
  seed, `generator.Complete` (`Workers: 1`, `MaxAttempts` still 300 per
  seed), rate, send, repeat. The search ends when *k* distinct puzzles are
  found (*k* from the dropdown "Höchstens *k* Vorschläge": 1, 2, 4, 8, 12,
  16, 24; default 8), when `MaxTries` = 100,000 generator attempts are used
  up in total, or when the page stops it ("Anhalten", closing the page).
  The dropdown is an upper limit: a pattern the generator rarely completes
  can end with fewer.
- **Counters.** A new `generator.Options.Tried` hook counts attempts; it
  does not change any result, so seeds and saved files reproduce as
  before. Every 500 ms the page gets tried / accepted (uniquely solvable and
  new) / duplicates, shown next to the time.
- **Brute force works, the rules only rate.** Uniqueness is decided by the
  brute-force solver inside the generator. The rule-based solver (with
  search, as before) rates only the *k* puzzles found, for the order on the
  page (hardest first) and the numbers under each.
- **`design` sets `GOGC` to 400** (`debug.SetGCPercent`). With 15 workers
  allocating at once, garbage collection was the largest single cost.

## Measurements

16 cores, 15 workers, *k* = 8, GOGC 400, three runs per pattern: time to the
first and to all 8 proposals, and the attempts used.

| pattern | first | all 8 | attempts |
|---|---|---|---|
| `n08_sad_smiley` | < 0.01 s | < 0.01 s | 21–34 |
| `n10_confused_smiley` | 0.01–0.02 s | 0.04–0.5 s | 153–281 |
| `n10_symmetry` | 0.03–0.1 s | 0.3 s | 5,700–6,800 |
| `n12_inseln` | 0.01–0.06 s | 0.3 s | 730–820 |
| `n14_stairs` | 1.8–2.1 s | 3.4–11 s | 5,700–6,800 |
| empty 14×14 | 0.01–0.03 s | 0.05 s | 16–24 |

`n10_symmetry` and `n14_stairs` show what the budget is for: about 800
attempts per puzzle, more than the 300 a seed gets, so before most of their
seeds failed (`n10_symmetry`: 11 of 20 seeds succeeded).

Attempts per second with the whole budget (15 s per pattern, measured while
the search still ran to the end, see Rejected):

| pattern | GOGC 100 | GOGC 400 |
|---|---|---|
| `n08_sad_smiley` | 5,695 | 11,855 |
| `n10_confused_smiley` | 2,065 | 4,472 |
| `n12_inseln` | 1,056 | 2,268 |
| `n14_stairs` | 725 | 1,323 |
| empty 10×10 | 773 | 1,614 |
| empty 14×14 | 80 | 141 |

So the budget lasts about 10 s at 8×8 and 20 to 80 s for drawn patterns of
10×10 to 14×14 when the generator rarely succeeds.

## Rejected

- **Search to the end of the budget and keep the k hardest.** Built first:
  the server kept the *k* hardest distinct puzzles, rated every new one
  with the rules alone and ran lookahead and guessing only if the rules got
  stuck and the puzzle could still enter; ties broke by order of entering,
  so the page's "sort, keep *k*" matched the server. It found harder
  puzzles (e.g. `n10_confused_smiley`: the best stuck at 86 free cells with
  28 lookahead crosses), but rating every puzzle took 40–60 % of the CPU on
  small or empty boards (2–8 % on drawn 10×10 to 14×14), and every search
  ran the whole budget. Difficulty is secondary for motifs: the drawing is
  what counts, so the search stops at *k*.
- **Base rules as a cheaper filter before all rules** (for the variant
  above). The base rules (`observers.BaseConfig`) cost about as much as all
  rules (8×8: 0.87 vs. 0.82 ms, 14×14: 4.8 vs. 7.7 ms), because the sync
  runner rarely gets to the look-ahead rules while the base ones keep
  finding something.
- **A time limit instead of an attempt budget.** Attempts make runs
  comparable across machines; the page shows the time and can stop at any
  moment.
- **Raising only the 300 attempts per seed.** They decide when drawn regions
  may grow (`generator.Complete`'s second round) and are part of what a
  saved seed means; the budget is the limit of the search instead.
