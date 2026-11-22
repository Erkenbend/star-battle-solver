# Architecture

This directory explains how the project is organised and why, for whoever
picks it up again (human or agent) without the commit history, chat sessions
or agent memory at hand. It stays at the level of purposes and boundaries; the
code and its comments carry the details.

The user documentation lives in [`docs/`](../docs) and, like the whole UI, is
written in German. Code, comments, scripts and this directory are in English.

| File | Content |
|---|---|
| this file | what the system is, how it is cut into parts, where state lives, how it is tested and shipped |
| [concurrency.md](concurrency.md) | a tour of every goroutine and channel, the patterns behind them and why each has its shape, with sequence diagrams and a step-through player |
| [refactorings.md](refactorings.md) | a review of the architecture with suggested (not yet done) refactorings |
| [decisions/](decisions/README.md) | one file per decision, including rejected ideas, with the reasons and measurements |

Related documents:

- [`README.md`](../README.md): what the project is, for players (German).
- [`docs/development.md`](../docs/development.md): build, test, and the developer tools `design` and `hardsearch` (German).
- [`PLAN.md`](../PLAN.md): the rules, their priority levels and measurements, the current state and open ideas (German). Read it, and the matching entries in [`decisions/`](decisions/README.md), before changing a rule, the generator or the search: most "why not …?" questions are answered there.
- [`grober_plan.md`](../grober_plan.md): the original idea, the only human-written text. Keep it unchanged.

## 1. What the system is

A Star Battle ("Meowdoku", "Queens") **solver that reasons like a human** and
explains every step in German, plus everything built around it so people can
use it: a puzzle generator, a terminal UI, and a browser game with a library of
hard puzzles and multiplayer "parties".

Variant: an N×N board with N regions ("colors"), exactly **one star** per row,
column and region, and stars never touch, not even diagonally.

The goal was never speed. Brute force solves any puzzle in milliseconds. The
goal is a **chain of understandable deductions**, simplest first, and a
justification for every cross and star. Everything else exists to produce
puzzles for that solver, to check that it is right, or to show its reasoning
to people.

It is also a Go experiment: the rules can run as concurrent observers, one
goroutine per rule, exchanging marks over channels (the premise of
`grober_plan.md`). [concurrency.md](concurrency.md) walks through this and
every other use of goroutines and channels.

## 2. Principles that shape the code

These hold across the code base. Changes that break one of them usually break
something far away.

1. **Monotone board.** A cell only goes from *free* to *star* or *cross*,
   never back. Hence the order in which deductions arrive does not matter,
   duplicates are harmless, every rule can work on its own copy of the board
   without locks, and both runners reach the same fixpoint. The browser relies
   on it too: a hint is valid as soon as the trace steps before it are on the
   board.
2. **Rules are pure functions.** A rule (an *observer*) looks at a board and
   returns marks with a reason. It knows nothing about goroutines, scheduling
   or search. Concurrency lives in the runners only, so each rule can be tested
   with a plain table test.
3. **Simplest explanation wins.** Each rule has a priority level. A rule may
   only fire when no simpler one finds anything; lookahead ("what if a star
   were here?") comes after all rules, real guessing comes last. Among
   refuted assumptions, the shortest chain is taken. This is what makes the
   trace read like a person thinking.
4. **Brute force is the referee, never the explanation.** A separate,
   independent backtracking solver decides uniqueness for the generator and
   serves as the test oracle for every rule. It never appears in a trace.
5. **Reproducibility.** Same size and seed give the same puzzle on every
   machine and with any number of workers. The explanation shown to people is
   produced by the deterministic runner. Puzzle URLs and file names rely on
   this.
6. **One binary, no dependencies.** No third-party Go modules, no JavaScript
   build, no CDN, no database. Pages, styles, scripts, the puzzle library and
   the symbols are embedded into the binary. A page saved with `starbattle
   html` is a single file that works offline.
7. **German for people, English for code.** All user-visible text (traces,
   UI, user docs) is German. Identifiers, comments, scripts, file names and
   this directory are English.

## 3. The parts

```
 commands     cmd/starbattle
              generate · show · solve · html · serve · design (dev) · hard (dev)

 delivery     internal/render     internal/web ──────────► internal/party
              (terminal)          (pages, site, library,   (the party game,
                                   party HTTP)              no HTTP)
                                       ▲
              internal/designer ───────┘      internal/hardsearch
              (`design`: draw, complete,      (`hardsearch`: find hard puzzles,
               save motifs)                    rate them)

 content      internal/collection: format of testdata/hard and designs/
              root package `starbattle` (embed.go): embeds testdata/hard,
              designs/, symbols/
              internal/sprite (dev): draws the animated sheets in symbols/
              internal/icon (dev): draws the web app's icons in
              internal/web/assets/icons
              internal/concviz (dev): draws the diagrams of concurrency.md
              internal/buildinfo: commit and build time of the binary

 reasoning    internal/solver ◄──── internal/solver/observers
              (runners, search,     (the rules + German text)
               trace)

 production   internal/generator ──► internal/bruteforce
              (unique puzzles,       (referee / oracle)
               also around a pattern)

 model        internal/board ──► internal/puzzle
              (marks, implied      (layout, units,
               crosses, conflicts)  text format)
```

Dependencies only point downwards: a package uses packages in its own row
(along the arrows) or in rows below it, never above. `puzzle` knows nothing
else; `board` adds state; `solver` adds deduction mechanics; `observers` adds
the actual rules.
The generator deliberately depends on brute force, not on the solver (it was
tried and was slower, see [decision 5](decisions/05-generator-uses-brute-force.md)).

### 3.1 Model: `puzzle` and `board`

`puzzle` is the immutable layout (which region each cell belongs to), the
*units* (every row, column and region: the things that need exactly one star)
and the text format (one letter per cell, `#` comments). `board` is a mutable
snapshot of marks on a puzzle, with the knowledge of what a star implies
(crosses in its row, column, region and neighbourhood) and detection of
contradictions (a unit with no room left, two stars in conflict). The
contradiction carries the cells involved, so it can be explained and shown.
What follows from the layout alone (the units, the crosses a star on each
cell implies) `board.New` computes once; clones share it read-only, so a
puzzle must not change once a board exists for it.

### 3.2 Reasoning: `solver` and `solver/observers`

The split is "mechanics" versus "knowledge":

- `solver` defines what a mark and an observer are, how a mark is applied, and
  how observers are driven to a fixpoint. It also owns the **search** on top of
  the rules (lookahead, then guessing) and the **trace**: every step with its
  rule, priority level, reason, marks, and for assumptions the nested chain
  that led to the contradiction.
- `observers` holds the rules themselves, their configuration and their German
  wording. Adding a rule means adding an observer type, giving it a priority
  level and registering it; the runners need no change. The rules, their
  levels are documented in `PLAN.md`, why each exists in [`decisions/`](decisions/README.md).

There are **two runners with identical semantics**:

- the *sync* runner calls the observers in priority order and restarts after
  every change. It is simple and deterministic, so it produces the
  explanations people see (`html`, `serve`, and all trial runs inside
  lookahead);
- the *concurrent* runner gives every observer its own goroutine and mailbox
  and detects the end via version numbers. It is the original experiment and
  the default of `starbattle solve`. A test asserts that both runners reach
  the same result and that the concurrent one honours the priority levels.

### 3.3 Puzzle production: `generator`, `bruteforce`, `hardsearch`

- `generator` makes puzzles with exactly one solution: random solution, grow
  regions around it, then repair until brute force finds no second solution.
  It can also *complete* a partially drawn pattern (used by the designer for
  picture puzzles). Several attempts run in parallel, but the result depends
  only on the seed.
- `bruteforce` is the independent oracle (see principle 4).
- `hardsearch` scans seeds for puzzles that defeat the rules and computes the
  1–3 star difficulty rating shown in the library. It powers the developer
  command `starbattle hardsearch`.

### 3.4 Delivery: `cmd/starbattle`, `render`, `web`, `party`, `designer`

`cmd/starbattle` is a thin dispatcher: one subcommand per use, flag parsing,
and wiring of the packages. The solver flags are defined once and shared by
every command that solves. `render` draws boards in the terminal.

`web` is the browser side. Its parts:

| Part | Purpose |
|---|---|
| page rendering (`web.go`, `assets/`) | turns a puzzle + solution + trace into **one self-contained HTML page**: template, CSS, JS and the data as inline JSON. `html` writes it to a file, `serve` sends it. |
| site (`server.go`, `pool.go`, `tutorial.go`) | the routes of `serve`: generated puzzles under stable URLs, pasted puzzles, the tutorial. The pool keeps one solved puzzle per size ready so "new puzzle" is instant. Every solve of `serve` goes through one budget (`Budget`), which keeps a slot for pages and lets a page's solve end when its visitor leaves. |
| library (`library.go`) | the embedded collections (hard puzzles, motifs) as browsable pages; each puzzle is solved lazily in the background on first visit. |
| parties (`party_server.go`) | HTTP and server-sent events of the party mode, and the page of a round. The game itself is `internal/party` (below). |
| settings / symbols (`symbols.go`) | the catalog of star symbols players can pick. |
| web app (`install.go`) | manifest, icons (files drawn by `internal/icon`) and the head tags that make the served site installable; the `html` page gets none of them ([decision 26](decisions/26-installable-web-app.md)). |

`designer` is the developer tool behind `starbattle design`, deliberately not
part of `serve`: draw a pattern, let the generator complete it on all cores
but one until there are the asked number of distinct puzzles (or a budget
of attempts is used up, see
[decision 29](decisions/29-designer-searches-longer.md)), give the regions of
one other colors if wanted, save it to a collection. Its page is built like the site's pages
(`web.ServePage`, with the shared style and `logic.js`) from its own assets, and
it reuses `web`'s play page for "try it".

`party` is the multiplayer game on its own: parties, players, rounds in three
modes (against each other, together on one board, against the clock),
presence, ranking. It is a state machine in memory behind one mutex and knows
neither HTTP nor pages: `web` gives it puzzles (`Config.Prepare`, from the pool
or the generator), carries actions and snapshots over HTTP and server-sent
events, maps its error kinds to status codes, and builds a round's page from
`Manager.Page`. Tests drive it with a fake clock.

The browser code is plain JavaScript without a build step, inlined into
each page by the template's two script slots: shared scripts and the page's
own. `logic.js` is the pure `Logic` module (board rules, hints, coop
operations, region colours, symbols, …), shared by every page and also run
under node by a Go test. The puzzle page's UI is `app.js`, the core (board,
play mode, undo, hints, clock, storage, options), plus one file per part:
`hypothesis.js`, `watch.js` (the solving path), `tutorial.js` and `round.js`
for party rounds, with `coop.js` and `race.js` for their modes. `boot()` in
`app.js` creates each part with the page object `P`, through which the parts
read and change the play state; the core calls back into them (`H.change`,
`party.controls`, …). The library, settings, party and designer pages have
their own small scripts instead.

The puzzle page is laid out for phones and tablets first (see
[decision 25](decisions/25-mobile-first-page.md)): below the board only the
star tool, ⓘ and the clock; above the board a strip with a tile per color
that shrinks to a dot once the color has a star (`#colors`, drawn with the
board, so it follows the solving path too; of fixed height, see below); the buttons of the shown tab in an action bar
(`.actionbar`), below the board up to 860 px and next to it above; what is
used now and then in "Mehr" (`#more-sheet`), opening in place below the bar.
Up to 860 px the site navigation of every page is a tab bar fixed at the
bottom. Pages change only through it; on the page, the only way on is the
next puzzle, below the banner once solved (`#next-puzzle`,
[decision 30](decisions/30-ways-between-pages.md)). Element ids
stayed what they were, so the parts and the UI check find their buttons
wherever they now sit. The other pages share its parts, all in `style.css`:
ⓘ beside the title (`#info-btn`) folds the page's longer text (`#info`);
an option is a switch (a checkbox in `.toggle` or `.option`), several sizes
are chips (`.sizes .size`), one of a few is a segmented choice (`.tabs`,
`.modes`), a group of choices has a small capital heading.

Three rules of the puzzle page that bugs found on phones broke:
`renderPlay` runs after every change, also from party events and timers,
and draws board and banner only in the play tab (the solving path draws its
own); a party page changes pages only through `Party.goTo`, as setting
`location` again while a page loads starts the load over; and nothing above
the board changes height on a tap, so the board stays where it is on
screen: the banner and the tutorial's card (`.news`) come below it on
phones and beside it on wide screens, a tab that comes later is greyed out
rather than hidden ([decision 28](decisions/28-news-below-the-board.md)).
Some board
logic exists twice, in Go and in JS; the comments point to the counterpart,
and the node test checks that following the hints solves every hard puzzle.

## 4. Where state lives

There is no database, on purpose. Knowing where each kind of state lives
explains most of the design:

| State | Where | Lifetime |
|---|---|---|
| Puzzles and their order in the library | text files in the repo (`testdata/hard`, `designs`), embedded at build time | forever; only ever appended to |
| A generated puzzle | nowhere: recomputed from size + seed (the URL) | stateless |
| Solver results | computed per request; cached in memory for the pool, the library and party rounds | process lifetime |
| Parties, players, rounds, the shared coop board | server memory behind one mutex | expire after inactivity; lost on restart |
| A player's progress, timer, solved library puzzles, preferences, chosen symbol, party token | the browser's `localStorage` | per device |
| The unsolved solo puzzle `/` leads back to | a cookie (`starbattle.solo`), set and cleared by the page, read by the server ([decision 27](decisions/27-solo-resumes.md)) | per device, 90 days |
| The party `/party` leads back to | a cookie (`starbattle.party`: code and the player's token), set and cleared by the party's pages, checked against the party by the server ([decision 31](decisions/31-party-resumes.md)) | per device, 7 days or until left |

Consequences worth knowing:

- **Library numbering is append-only.** A puzzle's display name ("10-3") is
  its position among puzzles of its size in `bibliothek.idx`. Links and saved
  progress use a hash of the region layout instead, because the old seeds no
  longer reproduce the old puzzles with today's generator. Never reorder or
  delete lines in an index file.
- **Generator changes change puzzles.** Any change to the generator or its
  defaults gives the same seeds different puzzles. That breaks old `/?n=…&seed=…`
  links but not the library, whose files are fixed data. A test checks that
  `hardsearch` still finds a known hard puzzle at a known seed, so such a change is
  noticed.
- **Trust model.** Pages carry the solution and the full trace; the UI
  decides what to reveal (no help in the library, no solution before the end
  of a party round). The server re-checks solutions only where results are
  shared between players (parties). This is friendly play, not anti-cheat.
- **Restarting the server ends all parties.** Acceptable for a home server;
  persistence would be the first thing to add if that changes.

## 5. Content directories

The binary is the data. Three directories at the module root are embedded by
the tiny root package `starbattle` (`embed.go`); it sits at the root only
because `go:embed` cannot reach parent directories.

- `testdata/hard`: puzzles that defeat the basic rules. It is **both** the
  regression corpus of the solver **and** the "hard" half of the browser
  library. `bibliothek.idx` gives the library order, `sample.idx` the subset
  the tests run by default (locally and in CI). Each file's header says which command and
  commit produced it and carries its difficulty rating, so the server does not
  compute ratings on start.
- `designs`: picture puzzles ("motifs") drawn with the designer, the other
  half of the library.
- `symbols`: the star symbols to choose from. The animated sprite sheets are
  drawn by `internal/sprite`; its test keeps the checked-in files and their
  `frames` and `clips` in `symbols.json` in step with the code.

The icons of the web app (`internal/web/assets/icons`) work the same way:
`internal/icon` draws them, its test keeps the PNG files in step, and `web`
embeds the files, so the server never draws.

`testdata/hard` and `designs` share one on-disk format, owned by
`internal/collection`: the index, file names (`nNN_STEM.txt`), header lines
(`# rating:`, `# name:`), the region-based ID, duplicate checks, and creating
files without ever overwriting. Everything that reads or writes these
directories (the library, `hardsearch`, the designer) goes through it, so the
append-only rule and the header format are defined in one place.

New content comes from the developer tools (`starbattle hardsearch -o testdata/hard`,
`starbattle design`) and is then appended to the index file by hand (the
designer appends for you). The workflows are in `docs/development.md`.

## 6. Testing strategy

- **Soundness against the oracle:** every deduction of every rule, on many
  generated puzzles and states, must agree with the brute-force solution.
- **Runner equivalence and priority order:** sync and concurrent reach the
  same state; no step fires while a simpler rule would have applied.
- **Hard corpus:** the puzzles in `testdata/hard` stay unsolvable for the
  basic rules and solvable with search; their stored ratings match today's
  rating. Locally and in CI only `sample.idx` runs (29 puzzles, every size, rule
  and rating); `STARBATTLE_ALL_HARD=1` runs all of them, which takes minutes
  locally and most of an hour in CI ([decision 22](decisions/22-ci-sample-only.md),
  [23](decisions/23-size-15.md)).
- **Rules in isolation:** table tests on hand-written board states.
- **Web:** party state machines with a fake clock, HTTP and SSE end to end,
  page data and escaping; the JS `Logic` module under node (skipped if node is
  missing).
- **UI check** (`internal/web/uicheck`): a node script drives a headless
  Chromium over the DevTools protocol through every mode of the puzzle page
  (solo, hints, checks, hypothesis, solving path, tutorial and tour, library,
  a page saved as a file, and parties against each other (also with lives),
  together and against the clock, with a second player via the API) and the
  designer, and records what the page and the server show after each action.
  The scenarios come in groups by page or way between pages (`groups/`, one
  file of observations each in `expected/`): after changing one page, its
  group is enough. The party and tutorial scenarios run on a phone-sized
  screen, the party's transitions (solving path, next round, next puzzle)
  under a stream of states and on a slow connection. In every scenario a page
  load the page aborts itself, a link on a puzzle page to another page
  beside the tab bar (other than the next puzzle) and, on the phone, the
  board moving under a tap are errors (in the designer, a desktop tool, its drawing and big picture
  moving under a click too); on the phone every observation also records whether the action
  bar is under the tab bar. Values that differ between runs (times, random
  party puzzles, the designer's completions) are normalized. The result must
  equal `expected/`; after an intended UI change, `run.sh -update` rewrites
  the files of the groups run and their diff is part of the review. Not part
  of `go test`: run it with `internal/web/uicheck/run.sh [group…]` (all groups
  about two minutes); CI runs all of them as its own job.
- **Concurrency diagrams:** the stories behind the diagrams and the player
  of [concurrency.md](concurrency.md) are checked against the source with
  `go/ast`, and the generated files against what the stories draw.
- **CLI:** smoke tests call the commands directly.

Day to day, run `go test -short ./...`, and the UI check after changing the
browser code. The full suite with every hard puzzle is CI's job.

## 7. Build, CI and deployment

- `go build ./cmd/starbattle` is the whole build. `go build` (not `go run`)
  matters for `hardsearch`, which writes the commit into the files it creates,
  and for the settings page, which shows it. The build time is not in Go's
  stamp: `deploy.sh` and the CI archives set it with `-ldflags -X`
  (`internal/buildinfo`); without it the page shows the commit's time.
- **CI** (`.github/workflows/ci.yml`) on every push: vet, the full test suite
  with all hard puzzles and coverage, a race run, cross-compiled archives for
  Linux (incl. Raspberry Pi), macOS and Windows as workflow artifacts, and on
  `main` the badge data (coverage, library size) pushed to the orphan branch
  `badges`, which the README badges read. There are no releases; the latest
  successful CI run is the download.
- **Pages** (`.github/workflows/pages.yml`): when the concurrency player
  changes on `main`, it is published on GitHub Pages
  (<https://erkenbend.github.io/star-battle-solver/>), the only thing there.
- **Deployment** (`deploy/`): a script cross-compiles for a Raspberry Pi,
  copies the binary and installs it as a systemd service. TLS and access
  control (the public instance uses basic auth) belong to a reverse proxy in
  front, not to the binary. Browsers install the site as an app only over
  HTTPS.

## 8. Where to start for typical changes

| Change | Start at | Keep in mind |
|---|---|---|
| a new deduction rule | `internal/solver/observers` | priority level, German wording, soundness test; rewrite the traces of the sample (`go test ./internal/web -run TestTraceGolden -update`) and review their diff; measure its effect on `testdata/hard`, update the rules in `PLAN.md` and record the measurement as a new file in `architecture/decisions` |
| solver search behaviour | `internal/solver` (search) | explanations must stay reproducible; the sync runner drives trial runs; `TestTraceGolden` (`internal/web/testdata/traces`) shows which explanations change |
| generator | `internal/generator` | changes all seeded puzzles; rerun the hard search test; measure (see decisions 8, 12, 13) and record it in `architecture/decisions` |
| more hard puzzles | `starbattle hardsearch`, then append to `testdata/hard/bibliothek.idx` | continue from the seed ranges in [decision 17](decisions/17-library-over-100-sample-tests.md) and `PLAN.md` |
| the format of puzzle files or index | `internal/collection` | the files are long-lived data: stay compatible with what is in the repo |
| a page or a UI feature | `internal/web/assets`, then the matching Go file in `internal/web` | logic that can be pure goes into `Logic` in `logic.js` with a node test, UI of the puzzle page into `app.js` or the file of its part; text in German; a new page template needs `<!--{{APP}}-->` in its head (a test checks); the UI check's group of the page (`run.sh <group>`, see `GROUPS` in `uicheck.mjs`), a new page gets one |
| party modes | `internal/party` (`party.go`: shared lifecycle, `coop.go`, `clock.go`), then `internal/web/party_server.go` (HTTP/SSE, round pages) and `assets/party.js` | state is in memory behind one mutex; tests use a fake clock |
| a star symbol | `symbols/`; animated ones in `internal/sprite` | see `symbols/README.md`; rewrite the sheets with `go test ./internal/sprite -update` |
| the app icon | `internal/icon` | rewrite the files with `go test ./internal/icon -update`; look at them at 32 px too |
| a CLI command | `cmd/starbattle` | document it in `docs/commands.md` |
| goroutines or channels anywhere | the section of the place in `architecture/concurrency.md` | update that file; run `go test -race` on the package; `go test ./internal/concviz` names the claims of the section's story that no longer hold: fix the story in `internal/concviz`, then `go test ./internal/concviz -update`; a new place gets a section and a story |
