# 4. New rules from recurring assumptions: `ForcedPartner` and `DeadCells`

- **Status:** done
- **Date:** 2026-11-04 (`56ddfa5`)

The plan: collect the assumption chains of the hard puzzles, group them by
pattern, and turn the most frequent patterns into observers with an
understandable sentence. Progress measured with `TestHardPuzzles`.

Implemented as `ForcedPartner` and `DeadCells`. They are the two most
frequent patterns in the assumption chains of the hard puzzles (together a
good three quarters of all chains). Both are configurable
(`observers.Config`, `solve -partner-depth N -dead-max N`, 0 switches a rule
off); `BaseConfig` keeps only the first three rules (`UnitFinish`,
`Confinement`, `Elimination`).

Result: with `ForcedPartner` and `DeadCells`, 31 of the 32 puzzles then in
`testdata/hard` are solved without assumptions; only `n11_s095` still needs
lookahead. The current state of all rules is in `PLAN.md`, section "Regeln".
