# 20. Difficulty of the hard puzzles

- **Status:** done
- **Date:** 2026-11-14 (`f314737`)

1 to 3 stars (`hardsearch.Rate`) from the solving path with full search:
★★★ for guessing, ≥ 10 lookahead steps or a chain of ≥ 7 steps; ★★ for
lookahead or `Pigeonhole`; otherwise ★. In the library this gives 31 / 46 /
41.

Stored as `# rating:` in every file of `testdata/hard` (written by
`starbattle hardsearch`, re-rate with `hardsearch -rate -o testdata/hard`,
checked by `TestLibraryRatings`), shown on the tiles, above the board and in
the shared result. Motifs are not rated.

Chosen after a measurement over all 118: no puzzle needs guessing, and the
chain length (3 to 8) separates less than the number of lookahead steps (1 to
53).

Open: a difficulty for generated puzzles (`PLAN.md`, "Später").
