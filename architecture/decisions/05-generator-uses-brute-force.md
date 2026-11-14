# 5. The generator checks uniqueness with brute force, not with the solver

- **Status:** rejected
- **Date:** 2026-11-04 (`477f593`)

Idea: let the generator use the project's own solver instead of brute force
to check that a puzzle has exactly one solution, with a CLI flag to switch
back to brute force.

Tried and rejected: brute force is faster when generating. The generator stays
with `bruteforce.Solve`. This is also why `internal/generator` depends on
`internal/bruteforce` and not on `internal/solver` (see section 3 of
[the architecture overview](../README.md)).
