# Decisions

One file per decision: what was done or rejected, why, and the measurements
behind it. Most "why not …?" questions are answered here, so read the
matching entries before changing a rule, the generator or the search.

The entries used to be the section "Erledigt / verworfen" of
[`PLAN.md`](../../PLAN.md), which now keeps only the rules, the current state
and open ideas.

| # | Decision | Status |
|---|---|---|
| [1](01-german-explanations.md) | Explanations in German, coordinates from 0 | done |
| [2](02-sync-runner-for-trial-runs.md) | Trial runs of the lookahead use the sync runner | done |
| [3](03-hard-puzzles-are-fixed-data.md) | The files in `testdata/hard` are fixed data | done |
| [4](04-rules-from-recurring-assumptions.md) | New rules from recurring assumptions: `ForcedPartner`, `DeadCells` | done |
| [5](05-generator-uses-brute-force.md) | The generator checks uniqueness with brute force, not the solver | rejected |
| [6](06-priority-levels.md) | Priority levels for the observers | done |
| [7](07-contiguous-confinement.md) | `Confinement` over adjacent rows and columns | done |
| [8](08-generator-repair-by-pieces.md) | Generator: repair by moving region pieces | done |
| [9](09-pigeonhole-rule.md) | New rule `Pigeonhole` | done |
| [10](10-rejected-shortcut-rules.md) | Rules "three columns of room" and "row/column intersection" | rejected |
| [11](11-html-page-and-serve.md) | HTML page and `serve` | done |
| [12](12-min-region-size-2.md) | Minimum region size 2 from N = 10 on | done |
| [13](13-parallel-generator-attempts.md) | Generator: attempts in parallel | done |
| [14](14-party-mode.md) | Playing together (party mode) | done |
| [15](15-hard-testdata-for-all-rules.md) | New test data that is hard even for the new rules | done |
| [16](16-user-documentation.md) | User documentation: README with the commands, later split up | done |
| [17](17-library-over-100-sample-tests.md) | Library over 100 puzzles, tests only with a sample | done, CI part replaced by [22](22-ci-sample-only.md) |
| [18](18-hard-command.md) | The search for hard puzzles as a command | done |
| [19](19-badges-and-coverage.md) | README badges and test coverage | done |
| [20](20-difficulty-rating.md) | Difficulty of the hard puzzles | done |
| [21](21-library-175.md) | Library with 175 puzzles | done |
| [22](22-ci-sample-only.md) | CI runs only the sample of hard puzzles | done |
| [23](23-size-15.md) | Size 15 in the game and the library | done |
| [24](24-concurrency-animations.md) | Diagrams and a step-through player for `concurrency.md`, checked against the code | done |
| [25](25-mobile-first-page.md) | Puzzle page laid out for phones first (variant B) | done |
| [26](26-installable-web-app.md) | The site can be installed as a web app (manifest, icons, no offline play) | done |
| [27](27-solo-resumes.md) | "/" leads back to the unsolved solo puzzle | done |
| [28](28-news-below-the-board.md) | Banner and tutorial card below the board, `keepBoard` removed | done |
| [29](29-designer-searches-longer.md) | Designer: search on all cores until *k* puzzles or a budget of attempts | done |
| [30](30-ways-between-pages.md) | Ways between pages only through the tab bar, one way to the next puzzle | done |
| [31](31-party-resumes.md) | "/party" leads back to the party one plays in, "Party verlassen" also in a round | done |
| [32](32-board-geometry.md) | Precomputed geometry for `board`, built by `New`, shared by clones | done |

## Adding a decision

- Take the next number; numbers never change, so they can be referred to.
  The file name is `NN-short-slug.md`.
- Start with the title, then **Status** (done, rejected, partly done, …) and
  **Date** with the commit(s). Then what was decided, the alternatives that
  were considered, and the measurements. Do not leave out numbers or rejected
  alternatives: they are the value of an entry.
- Things left open go into an "Open:" paragraph at the end. When one is taken
  up, it becomes a decision of its own that links back.
- If a later decision replaces an earlier one, change the earlier one's
  status to "replaced by N" (with a link) instead of deleting it.
- Add a row to the table above. Like the rest of this directory, entries are
  in English.
