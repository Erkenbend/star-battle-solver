# 35. Structured references in the trace

- **Status:** done
- **Date:** 2026-11-22 (`e6fc647` to `db984d9`, `14cc752`, and the commit of this entry)

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
  they cannot drift apart, and nothing parses the text. A rule hands
  `Mark.Why` a function that writes its text through a `Namer`, so it
  cannot give a reason without its refs.
- **Explanations are built when they are read.** A mark's `Reason`
  (`solver.Reason`) runs that function on first use, once, and keeps text
  and refs. Lookahead runs a trial for every free cell and keeps one chain,
  so most explanations are never written. `record` still merges marks with
  the same text: the same `Reason` needs no text, only two reasons of the
  same source are written to compare them. Elimination groups its crosses
  by cause (which unit holds the free cells, or how they touch the cell)
  instead of by text; the cause decides the text, so the groups are the
  same. The price: what a rule's function reads must not change after the
  deduction (the rules capture values they are done with; `confinedSets`
  hands out fresh maps), and traces no longer compare with
  `reflect.DeepEqual` (a `Reason` holds a function), so `TestSolveCancelled`
  compares their text.
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
  allocations). Not needed once explanations are built only when read, and
  lists map directly to the JSON.
- **Building refs eagerly with the texts**, as at first. It cost about 3 %
  more solver CPU, almost all in lookahead trials whose explanations nobody
  reads.

How it was checked: while both existed, `logic_test.js` asserted that the
old parser and the new refs give the same cells and units for every text
of the 29 sample traces plus the tutorial (steps, chain steps and
contradictions). That held, and the parser was removed. `TestTraceGolden`
and `solve -runner sync -verbose -nocolor` on all 193 puzzles of
`testdata/hard` stayed byte for byte the same, also with the lazy
explanations, as did the `html` pages measured below. The golden traces now
carry the refs too, so a change of what a step names, or of their order
from run to run, shows in their diff. UI check: 247 observations, no
difference.

Measurements (Ryzen 7 5800X3D, 16 threads; before is `dc26869`):

| | before | after | |
|---|---|---|---|
| `solve -runner sync -verbose`, all 193 of `testdata/hard`, sum of wall times (2 runs each) | 106.3 s, 107.1 s | 98.4 s, 98.5 s | −8 % |
| same, CPU (user + system) | 462.5 s, 466.7 s | 432.9 s, 434.8 s | −7 % |
| same, largest resident memory of one puzzle | | 38 to 43 MB | about the same |
| trace JSON of the 29 sample puzzles (estimated from the golden traces) | 526 KB | 624 KB | +18.5 % |
| `html` page, n15_s130 / n13_s4661 / n10_s047 | 201 / 253 / 174 KB | 206 / 267 / 175 KB | +0.5 to 5.5 % |
| same, `gzip -6` (`serve` uses `BestSpeed`, decision [34](34-gzip.md); the ratio is what matters) | 55.6 / 59.2 / 52.4 KB | 56.4 / 61.2 / 52.7 KB | +0.5 to 3.5 % |

By size (sum of wall times): N = 13 went from 33.2 to 31.1 s, N = 14 from
44.1 to 40.4 s, N = 15 from 20.5 to 18.8 s. So the solver got faster than
before refs existed: the texts the trials used to write were a cost of
their own. In the profile of `TestHardPuzzles` the explanations no longer
show, and the garbage collector's background work went from 3.5 to 2.6 s
of 21 s.
