# Architecture review and suggested refactorings

Two reviews: the first in early November 2026 (about 90 commits, ~18k lines
including tests and assets, suggestions 1 to 4, all done), the second on
21 November 2026 (suggestions 5 to 14, see "Second review"). The numbers of
the suggestions stay fixed so they can be referred
to. When one is done, move it to "Done" at the end in a sentence or two and
update [README.md](README.md) if the structure changed. When one is rejected,
note why, so it is not proposed again.

## Verdict of the first review

The core is in good shape and needs no restructuring: `puzzle` → `board` →
`solver` → `observers` is layered cleanly, dependencies only point downwards,
rules are pure and tested against an independent oracle, and the two runners
share all their semantics. The generator, brute force and hard search are
small, focused packages.

The pressure is at the edges, where features have accumulated:

- `internal/web` held over 40 % of all non-test Go code and mixed six
  concerns (page rendering, site routes, library, parties, symbols, the
  developer designer) (done, see 2).
- `assets/app.js` (~2,150 lines) was one file with the pure logic and the UI
  of every kind of page (done, see 3).
- The format of the puzzle *collections* (index files, header lines such as
  `# rating:` and `# name:`) was read and written in three different places
  (done, see 1).

All four suggestions of the first review are done (see "Done"). New ones go
in a numbered section of their own above "Guidelines", continuing after the
highest number in use (14).

## Second review (November 2026)

Review of 21 November 2026, about 190 commits: ~13,600 lines of non-test Go
(3,500 of them the diagram tool `concviz`) and ~4,100 lines of browser
JavaScript. Goal: code that stays maintainable and robust as features keep
coming, without getting slower.

The layering of the first review still holds: dependencies point
downwards, the rules are pure, `party` knows no HTTP, `collection` owns the
file format. The tests are strong (oracle, runner equivalence, priority
order, hard corpus, UI check with 6 groups) and make the refactorings below
low-risk. The pressure has moved:

- **Speed of the solver core.** Nobody has profiled it since it got the
  rules that look one star ahead, and allocation now dominates (5, 14).
- **The server has no budget for the solver.** The generator has one
  (`cpuSlots`); solving runs unbounded, cannot be cancelled and is not
  cached for most pages (6). Pages are sent uncompressed (7).
- **Go and JS are tied by German prose.** The page finds rows, columns,
  colors and cells by parsing the explanation text (8).
- **`app.js` is growing again** (about 850 lines after step 2 of 3, now
  1,123), and every new mode adds branches to `renderPlay` (9, 10).

| # | Suggestion | Gain | Cost | Risk |
|---|---|---|---|---|
| 5 | Precomputed geometry for `board` | speed: ~20–30 % of solver CPU | small | low (traces must stay byte for byte) |
| 6 | One solve budget in `serve`: limit, cancel, cache | robustness on the Pi | medium | low |
| 7 | gzip for pages and JSON | 208 → 56 KB per page | small | low |
| 8 | Structured references in the trace | decouples Go wording from JS | medium | low (existing guard test) |
| 9 | Puzzle page: one state object, capabilities instead of flags | growth of the puzzle page | medium | medium (UI check covers it) |
| 10 | Shared DOM and format helpers for all pages | less duplication | small | low |
| 11 | Split `party.go` like `coop.go` and `clock.go` | readability | small | very low |
| 12 | One function for "puzzle of size n, seed s" | protects principle 5 | small | very low |
| 13 | Small server and CLI hardening | robustness | small | low |
| 14 | Lookahead: stop trials that cannot win | speed | medium | medium (must stay deterministic) |

The numbers are the ranking: value for maintainability, robustness and
speed against cost and risk. 5 to 7 are worth doing soon; 9 and 10 go
together; 14 only after 5, and only if measurements still call for it.

## 6. One solve budget in `serve`: limit, cancel, cache

Every caller of the server's `SolveFunc` runs the full solver with
lookahead, which starts `GOMAXPROCS` workers per solve:

- the pool (one size at a time, fine);
- the library: the first visit of each puzzle starts a goroutine of its own
  (`Library.start`). A crawler or a curious player opening the ~190 hard
  puzzles one after another starts that many solves at once on the Pi;
- party preparation, one per party (up to `MaxParties`, against the clock
  several in a row);
- `/?n=…&seed=…` when the pool does not have it: generate and solve on
  every request, also on every reload of an older puzzle (the pool keeps
  only the last puzzle it handed out per size); `/play`; `/anleitung`
  solves its fixed puzzle on every request.

Generating is limited across the process (`generator.cpuSlots`), solving
is not, and `solver.Solve` cannot be cancelled: a solve for a request that
is gone runs to the end.

Proposal, in this order:

1. `solver.Options.Done` (like `generator.Options.Done`), checked between
   lookahead trials and before each guess; `Solve` then ends with a new
   status or error that callers treat like a failed request.
2. In `web`, a limiter around `SolveFunc` (a buffered channel as
   semaphore, sized like `cpuSlots`). Never hold it while waiting for
   `cpuSlots` or the other way round: generate first, then solve.
3. A small bounded cache of served results keyed by (n, seed) and by the
   `/play` text (e.g. the last 64), so reloads and shared links cost
   nothing; the tutorial's result computed once (`sync.Once`, as the
   library does).

New goroutines or channels: update `architecture/concurrency.md` and its
story in `internal/concviz` (pool, library and party sections).

## 7. gzip for pages and JSON

A puzzle page is about 208 KB (a 15×15 hard puzzle via `html`: 42 KB CSS,
~150 KB script, the trace), 56 KB gzipped. `serve` sends it uncompressed
and with `Cache-Control: no-store`, so every page change (next puzzle, the
next party round) loads all of it again. The deployment docs suggest Caddy
in front, which compresses only with an `encode` directive, and players on
the home network reach the binary directly. The UI check's slow-connection
scenarios show why this matters.

Proposal: compress in `writeHTML` and `writeJSON` (both already buffer the
whole answer) when the request accepts gzip, with `compress/gzip` from the
standard library (`BestSpeed` if the Pi needs it; measure). Not for the
event stream (`/party/{code}/events`, it must flush per event) and not for
the PNG icons. Single-file pages (principle 6) stay as they are. Test:
a request with `Accept-Encoding: gzip` gets a body that decodes to the
same page.

## 8. Structured references in the trace

The page highlights the rows, columns, colors and cells an explanation
names, and puts color swatches in its text, by parsing the German text:
`Logic.refs` and `Logic.colorSegments` match "Zeilen 3 und 4",
"Spalten 2 bis 5", "Farben A, B und D" and "(2,3)" with regular
expressions. A test in `logic_test.js` checks that every explanation of the
sample traces is understood, so it holds today, but every new wording
(e.g. "Zeilen 3–5"), a new rule or a second language has to keep the Go
text and the JS patterns in step.

Proposal: the rules already know what they name (`Confinement`'s from and
target sets, `ForcedPartner`'s chain, …). Give `solver.Step` (and
`Hypothesis`) a `Refs` value (cells, rows, columns, regions), filled where
the reason is built, sent compactly in `web.Step`, and used by the page
instead of `refs`. Swatches can use `Refs.Regions` to replace only the
letters that are referenced. While both exist, the node test can assert
that the old parser and the new refs agree on every sample trace, then
the parser goes. Measure how much the trace JSON grows.

In the same go, the German names of units and cells now live in three
packages: `board.Contradiction` (where "Zeile", "Spalte", "Farbe" also serve
as map keys for offsets), `solver` (`unitName`, the texts of `refute` and
`assumedStep`) and `observers/explain.go`. A small package below `board`
with these words (unit, units, cell, cell lists) would give every text one
source.

## 9. Puzzle page: one state object, capabilities instead of flags

`boot()` in `app.js` is one closure of 1,100 lines. The page object `P`
forwards 13 pieces of play state through getter and setter pairs
(`marks`, `done`, `hintCount`, `elapsed`, `runningSince`, `overlay`, …),
and every new piece of state a part needs adds another pair. The kind of
page is spread over flags (`tut`, `party`, `coop`, `race`, `lib`, `motif`,
`bare`, `soloPage`, `d.served`) that `renderPlay`, `drawClock`,
`drawSummary` and `playBanner` combine anew; `renderPlay` alone has about 15
of these branches.

Proposal, each step a commit of its own, checked with the UI check (all
groups that use the puzzle page):

1. The play state in one object `S` that `P` hands out (`P.S.marks`); the
   accessor pairs go.
2. The kind of page decided once, into an object of capabilities: hints,
   checks, own clock or the party's, storage key, what comes after solving,
   which tabs exist. The functions above ask it instead of combining flags.
3. Storage (keys, saved progress, the library's solved list, the solo
   cookie) and help (hints, checks, paid mistakes) as parts of their own,
   like `hypothesis.js`, which brings `app.js` back to about the core.

Still no modules, build step or framework (see 3 under "Done").

## 10. Shared DOM and format helpers for all pages

The same small helpers are written out several times: `el` in `app.js`,
`party.js`, `library.js`, `settings.js` and `designer.js`; `fmtTime` and
`hintsText` in `app.js` and `party.js`; `copyText` with its `execCommand`
fallback in `app.js` and `party.js`. Proposal: `fmtTime` and `hintsText`
into `Logic`, with node tests; `el` and `copyText` into a small `dom.js`
shared by every page with script. Note that the party page gets no
`logic.js` today (`WriteParty` leaves the shared slot empty): it would get
`dom.js` and `logic.js`, about 27 KB more, or only `dom.js`. Best done
together with 9.

## 11. Split `party.go` like `coop.go` and `clock.go`

`party.go` has 1,023 lines: limits and types, joining and leaving, the
actions of the mode against each other (progress, solve, lives, giving
up), the round's page, presence and ticks, preparing puzzles, and the
snapshot with the ranking (~170 lines). The other two modes already have
files of their own. Proposal, moving code only: `versus.go` (its actions
and lives), `prep.go` (`startPrep`, `prepTarget`), `snapshot.go`
(`Snapshot`, ranking); `party.go` keeps the types and the shared lifecycle.
`go test ./internal/concviz` names the claims about `startPrep` that move
(the story points to files).

## 12. One function for "puzzle of size n, seed s"

Principle 5 (same size and seed, same puzzle) rests on one convention,
`rand.New(rand.NewPCG(seed, uint64(n)))` plus
`generator.DefaultMinRegionSize(n)`, which is written out in
`cmd/starbattle` (`generate`), `web/server.go` (`generate`), `hardsearch`
and, for the random source, `designer` (whose saved header spells it out
too). A fifth copy that differs slightly would give the same URL a
different puzzle. Proposal: `generator.Seeded(n, seed)` for the random
source and `generator.ForSeed(n, seed)` for the puzzle, used everywhere;
a test that `generate -seed`, `/?n=&seed=` and `hardsearch` agree on one
seed.

## 13. Small server and CLI hardening

- `http.Server` in `serve` and `design` sets only `ReadHeaderTimeout`; add
  `IdleTimeout`. A `ReadTimeout` only together with
  `rc.SetReadDeadline(time.Time{})` in the event stream, as for the write
  deadline.
- `cmdServe` and `cmdDesign` repeat the block that listens, waits for a
  signal and shuts down: one helper.
- `hardsearch -n` checks against the literal 52 instead of `puzzle.MaxN`.
- `main.go` (520 lines) still holds `writeHard` and `rateDir`, the
  hard search's file output (left there on purpose in 1). If `hardsearch`
  grows, they belong next to `Search` in `internal/hardsearch`.

## 14. Lookahead: stop trials that cannot win

`lookahead` runs every trial to its end and then takes the refutation with
the fewest steps (then fewest cells, then the earliest candidate). The
trials are 37 % of the CPU in the profile of 5. A trial that already has
more steps than a refutation found by another trial can never be chosen,
so it could stop there, without changing which one is chosen: the
explanation stays the same and does not depend on scheduling. It needs
`RunSync` with a step limit (an internal variant) and a shared "best so
far" read by the workers. The candidates come tightest unit first, so
short refutations tend to be found early. Do it after 5, measure on
`testdata/hard`, keep it only if it pays, and record it as a decision with
the check that every trace stays byte for byte the same.

## Guidelines

What the review found that needs no change now, only attention later:

- **Duplicated rules in Go and JS** (implied crosses, coop operations, cell
  encoding): keep the cross-references in comments; if a third copy appears,
  consider generating constants from Go into the page data instead.
- **A puzzle does not change once a board exists for it.** The generator
  changes `Regions` in place while repairing a layout, before anything
  solves it; everything after that treats a `*puzzle.Puzzle` as read-only.
  The geometry `board.New` derives from it relies on that (see 5 under
  "Done"); a tool that edits a puzzle makes a new one (`Clone`).

## Considered and not recommended

- **Rename `testdata/hard`** to something like `library/hard`, since it is
  production content, not just test data. The name is surprising, but it is
  referenced from tests, CI, README badges, file headers and documentation,
  and Go treats `testdata` specially in a way that does no harm here. The
  explanation in [README.md](README.md) is cheaper than the rename.
- **Drop the concurrent runner** because the sync runner produces all
  explanations people see. It is the project's original premise, it is
  tested for equivalence, and it costs little to keep.
- **Use the solver instead of brute force in the generator.** Tried, slower;
  see [decision 5](decisions/05-generator-uses-brute-force.md).
- **Persist parties** (e.g. a file or SQLite). Not needed for a home server
  where a restart ending running parties is acceptable. It would also break
  the "no dependencies" property, and should be the first item if that
  changes.
- **Cache units and region cells on `puzzle.Puzzle`** (second review).
  `Regions` is exported and changed in place by the generator, so a cache
  there could go stale unnoticed. Derived tables belong to `board`, built
  when a board is made (see 5).
- **Serve the scripts and the style as separate, cacheable files**
  (second review). It would save more per page change than gzip (about
  150 KB of script are the same on every puzzle page), but it is a second
  way of delivering pages next to the single file of `html`, with cache
  versioning to get right. gzip (7) first; reconsider if pages grow well
  beyond 200 KB or slow connections still hurt.
- **One mutex per party instead of one for all** (second review). With at
  most 50 parties of 16 players, waiting for the lock is negligible, and one
  mutex keeps ticks, expiry and preparation simple.

## Done

- **1. One home for puzzle collections** (November 2026): `internal/collection`
  owns the format of `testdata/hard` and `designs` (index, file names, header
  lines, ID, duplicate checks, create-never-overwrite). The root package keeps
  only the embeds, `web` only the library pages, and the designer and `hardsearch`
  (still in `main.go`: its search loop, provenance header and output) call
  the package instead of knowing the format themselves.
- **2. Split `internal/web` along its concerns** (November 2026):
  - `internal/party` is the party game without HTTP or pages. It gets its
    puzzles through `Config.Prepare` and returns errors with a kind that `web`
    maps to status codes. It hands out `Manager.Page` (puzzle and round)
    instead of finished page data; `web` builds the page and decides that
    rounds against the clock get no help.
  - `internal/designer` is the `design` tool with its own assets. It uses
    `web.ServePage`, the only new export, and `web`'s play page.
  - `web` went from ~3,400 to ~1,450 lines of non-test code. The page data of
    party rounds is no longer cached per round (it is cheap to build); the
    wire format of the party mode is unchanged.
- **3. Split `app.js`, step 1: `Logic` into `logic.js`** (November 2026):
  the pure functions are their own asset, tested by `logic_test.js`. The
  template slots are now `SHARED` (logic.js, plus the UI on the puzzle page)
  and `SCRIPT` (the page's own). The library, settings and designer pages,
  which only need `Logic`, no longer carry the puzzle page's UI: about 67 KB
  less script each.
- **3. Split `app.js`, step 2: the puzzle page's parts** (November 2026):
  `app.js` (~850 lines) is the core; hypothesis mode, watch mode, the
  tutorial and party rounds (with coop and race) are files of their own
  (100 to 200 lines each). `boot()` creates them with a page object `P` whose
  accessors read and write its play state, so the core's code stayed as it
  was; it calls the parts through small objects (`H.change`, `W.render`,
  `party.controls`, …). Checked with a behaviour script over the DevTools
  protocol that plays every mode (solo, hints, checks, hypothesis, watch,
  tutorial and tour, library, file page, versus, coop, against the clock):
  the same 99 observations before and after. That script is now the UI
  check in `internal/web/uicheck`, run by CI.
  - Not done: a JS build step, modules loaded at runtime, or a framework.
    Single-file pages without a build (principle 6 in
    [README.md](README.md)) are worth more than the convenience.
- **4. Small clean-ups** (November 2026): `hard.go` is `embed.go`, the place
  to look for what is built into the binary. The trace printing of `solve`
  moved from `main.go` to `render.Trace` (writing to an `io.Writer`, with
  tests of its own); `solve`'s output stayed byte for byte the same (18
  flag and puzzle combinations compared). Of the two guidelines that were
  part of 4, one is under "Guidelines" above, the other is the next entry.
- **Decision log out of `PLAN.md`** (November 2026, a former guideline): the
  entries of its "Erledigt / verworfen" and "Entscheidungen" sections are now
  one file each in [decisions/](decisions/README.md), translated to English
  and not shortened. `PLAN.md` keeps the rules, the current state and open
  ideas.
- **5. Precomputed geometry for `board`** (November 2026): `board.New`
  builds the units, the implied crosses of every cell and a covers table
  once, clones share them read-only; `FreeCount` and `FirstFree` next to
  `Free`. Same output, the hard puzzles solve about 2.4× faster
  ([decision 32](decisions/32-board-geometry.md)).
