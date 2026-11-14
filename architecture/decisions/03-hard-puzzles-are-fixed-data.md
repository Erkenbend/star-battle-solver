# 3. The files in `testdata/hard` are fixed data

- **Status:** done
- **Date:** 2026-11-04 (`aacce67`), provenance extended up to `e76b5ce`

The CLI and the tests use the same seeding formula (`rand.NewPCG(seed, n)`).
Still, the files in `testdata/hard` are treated as **fixed test data**, not as
something to regenerate from their seeds: every change to the generator gives
the same seeds different puzzles.

Provenance of the files:

- The original ones come from the old generator (up to commit `def8c05`,
  e.g. `generate -n 5 -seed 29` → `n05_s029.txt`). With the new generator the
  same seeds give different puzzles.
- `n14_s002/024/029` come from commit `48ea102` (`generate -n 14 -seed 2`
  etc., `MinRegionSize` 2, attempts still one after another).
- Ten more (`n10_s396` to `n14_s025`) come from commit `46ab527` (attempts in
  parallel; the header line of each file names the command). See
  [15](15-hard-testdata-for-all-rules.md).
- The 73 newest (from `n10_s453` on in `bibliothek.idx`) come from commit
  `e76b5ce`; there the generator still produced the same puzzles as at
  `46ab527`. See [17](17-library-over-100-sample-tests.md).

Consequence for the library: names and links cannot rely on seeds. A puzzle's
display name is its position in `bibliothek.idx` (append only), links and
saved progress use `collection.ID`, a hash of the normalised regions (see
section 4 of [the architecture overview](../README.md)).
