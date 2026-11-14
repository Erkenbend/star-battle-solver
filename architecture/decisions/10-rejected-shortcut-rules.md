# 10. Rules "adjacent rows/columns with room in three columns" and "intersection of row and column"

- **Status:** rejected
- **Date:** 2026-11-05 (`50ef3e5`)

Both were planned as new rules and rejected: neither finds anything new. They
only combine chains of existing rules (`Elimination` ×2 + `Confinement` k=2,
respectively `Confinement` k=1 + `UnitFinish`), and to have any effect they
would have to rank before the simple rules.
