# 9. New rule `Pigeonhole`

- **Status:** done
- **Date:** 2026-11-05 (`50ef3e5`)

`Pigeonhole` is `Confinement` with one assumption: if after a star on c, k
open units have room in fewer than k units of another type, c is a cross. It
shares the search for subsets and blocks with `Confinement`
(`confinedSets`).

Measurement on `testdata/hard`:

- With all rules it fires 3 times, all in `n11_s095` (11 instead of 14
  lookahead steps); the puzzle still needs lookahead. `ForcedPartner` and
  `DeadCells` take precedence and cover almost everything.
- With the base rules only it is strong: 19 of 32 without lookahead
  (otherwise 0), 47 instead of 108 lookahead steps.
- k ≤ 3 gains nothing over k ≤ 2, hence the default maximum of 2.
