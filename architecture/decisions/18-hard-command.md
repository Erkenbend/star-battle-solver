# 18. The search for hard puzzles as a command

- **Status:** done
- **Date:** 2026-11-12 (`17c7900`)

`starbattle hard` (package `internal/hardsearch`, options in
[`docs/commands.md`](../../docs/commands.md)) replaces the throwaway program
of the earlier searches, which was never in the repo. The same criteria as
`TestHardPuzzles`, output per seed in seed order, with `-o testdata/hard`
directly as a file; `bibliothek.idx` stays manual work.

The test checks that seed 453 again gives `n10_s453.txt` with 4 lookahead
steps, i.e. that the generator and the rules still reproduce the library.
