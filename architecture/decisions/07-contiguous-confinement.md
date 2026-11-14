# 7. `Confinement` over adjacent rows and columns

- **Status:** done, now the default
- **Date:** 2026-11-05 (`def8c05`)

For k ≥ 2, `Confinement` over rows and columns considers only blocks of
consecutive open lines by default; `-non-contiguous` switches back to
arbitrary subsets. Solved lines inside a block are skipped. Explanations name
ranges: "Zeilen 3 bis 5".

Measurement over 900 generated puzzles (N = 4 to 12, including the 32 hard
ones):

- All rules: 899/900 without lookahead, 14 lookahead steps, with and without
  blocks alike.
- Base rules only: 867 instead of 868 without lookahead, 109 instead of 108
  lookahead steps.
- Gain: with arbitrary subsets, 41 of 208 `Confinement` steps (k ≥ 2,
  rows/columns, base rules, including assumption chains) used non-adjacent
  lines, e.g. "Zeilen 1 und 6 …".
- Blocks with k > N/2 never came into play.

Open: whether "adjacent regions" would be more human for colours. Together
with the rule "adjacent rows/columns with room in three columns" (see
[10](10-rejected-shortcut-rules.md)) this gives a family of rules for
adjacent lines.
