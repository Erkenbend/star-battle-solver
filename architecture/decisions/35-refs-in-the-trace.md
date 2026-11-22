# 35. Structured references in the trace

- **Status:** done
- **Date:** 2026-11-22 (`e6fc647`, `ab819fb`, `10be39b`, and the commit of this entry)

Suggestion 8 of [refactorings.md](../refactorings.md). The page highlighted
the rows, columns, colors and cells an explanation names, and put color
swatches in its text, by parsing the German text (`Logic.refs`,
`Logic.colorSegments`): every new wording, rule or language had to keep
the Go text and the JS patterns in step.

Decided:

- **`internal/words`**, below `board`, is the one source of the German
  names of units and cells ("Zeilen 3 bis 5", "Farbe B", "(2,3)") for
  `board` (contradictions), `solver` (assumption, refutation, guess) and
  the rules. Before, they lived in three packages.
- **A `Namer` records what it names.** It gives the same names and collects
  the units and cells in `Refs`. Text and refs come from the same calls, so
  they cannot drift apart, and nothing parses the text. `Mark.Why` takes
  both, so a rule cannot hand over a reason without its refs.
- **On the wire**, `web.Step` and `web.Hypothesis` (for the contradiction)
  carry `refs`: `{cells: [[r, c], …], rows, cols, regions}`, each omitted
  when empty. The page highlights from `Logic.refCells`. Swatches replace
  only the letters of named regions, so a stray "Farbe X" can no longer
  become one. The page's own texts (tutorial, problem list, banner) have no
  refs and keep a swatch for every "Farbe X" in them.
- **Sets of units in the order of the text.** The rules pass map keys, so
  recording them in the given order made the refs of the same step differ
  between runs. `TestSolveCancelled` found it (principle 5: same puzzle,
  same page). `Namer.Units` records them sorted, as the text lists them.

Considered:

- **Refs computed from the text in Go**, one parser instead of the JS one.
  It keeps the coupling, only on the other side.
- **Bitmasks instead of lists** for rows, columns and regions (fewer
  allocations). Not done, because the cost below is small, and lists map
  directly to the JSON.

How it was checked: while both existed, `logic_test.js` asserted that the
old parser and the new refs give the same cells and units for every text
of the 29 sample traces plus the tutorial (steps, chain steps and
contradictions). That held, and the parser was removed. `TestTraceGolden`
and `solve -runner sync -verbose -nocolor` on all 193 puzzles of
`testdata/hard` stayed byte for byte the same. UI check: 247 observations,
no difference.

Measurements (Ryzen 7 5800X3D, 16 threads; before is `dc26869`):

| | before | after | |
|---|---|---|---|
| `solve -runner sync -verbose`, all 193 of `testdata/hard`, sum of wall times (2 runs before) | 106.3 s, 107.1 s | 109.4 s | +2.5 % |
| same, CPU (user + system) | 462.5 s, 466.7 s | 483.8 s | +4 % |
| trace JSON of the 29 sample puzzles (estimated from the golden traces) | 526 KB | 624 KB | +18.5 % |
| `html` page, n15_s130 / n13_s4661 / n10_s047 | 201 / 253 / 174 KB | 206 / 267 / 175 KB | +0.5 to 5.5 % |
| same, `gzip -6` (`serve` uses `BestSpeed`, decision [34](34-gzip.md); the ratio is what matters) | 55.6 / 59.2 / 52.4 KB | 56.4 / 61.2 / 52.7 KB | +0.5 to 3.5 % |

By size (sum of wall times): N = 13 went from 33.2 to 34.5 s, N = 14 from
44.1 to 44.9 s, N = 15 from 20.5 to 20.8 s. In the profile of
`TestHardPuzzles`, building the refs costs about 3 % of CPU, mostly
`Confinement.explain`. The refs are built in lookahead trials too, whose
explanations are never shown. Sorting a set of units once instead of twice
(`ab819fb`→`10be39b`) took the first measurement from +3.5 % wall and
+5 % CPU down to the numbers above.

Open: the trials could build their texts and refs lazily, only for the
chain that is kept. That would save the 3 % and the cost of the texts
themselves, but it changes how a mark carries its reason.
