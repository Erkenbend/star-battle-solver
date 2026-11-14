# Architecture review and suggested refactorings

Review as of November 2026 (about 90 commits, ~18k lines including tests and
assets). The numbers of the suggestions stay fixed so they can be referred
to. When one is done, move it to "Done" at the end in a sentence or two and
update [README.md](README.md) if the structure changed. When one is rejected,
note why, so it is not proposed again.

## Verdict

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
  of every kind of page (logic split off, see 3; the UI remains one closure).
- The format of the puzzle *collections* (index files, header lines such as
  `# rating:` and `# name:`) was read and written in three different places
  (done, see 1).

The suggestions are ordered by value for effort. None is urgent; each is worth
doing when the area is touched next anyway.

## 3. Split the puzzle page's UI (`app.js`) into files (optional, only if it keeps growing)

**Now:** step 1, the pure logic in `logic.js`, is done (see "Done").
`app.js` is the puzzle page's UI: `boot()`, ~1,600 lines in one closure,
covering solo play, playback of the trace, hints, the hypothesis mode,
library puzzles, the three party modes and the tutorial.

**Suggestion (step 2):** cut mode-specific UI (party modes, tutorial,
hypothesis mode) out of `boot()` into separate files that plug into a small,
explicit interface of the page (board redraw, lock, storage key, toolbar). This is harder than it looks because the parts share
closure state; define the interface first, move second.

**Not suggested:** a JS build step, modules loaded at runtime, or a framework.
Single-file pages without a build (principle 6 in [README.md](README.md)) are
worth more than the convenience.

## 4. Small clean-ups

- **Rename `hard.go` to `embed.go`** (or `content.go`): it embeds `designs`
  and `symbols` too and is the place to look for "what is built into the
  binary".
- **Terminal trace printing** (`printSteps`, `describeMarks` in
  `cmd/starbattle/main.go`) belongs next to the board rendering in
  `internal/render`, again to keep `main.go` a dispatcher.
- **`PLAN.md` as a decision log:** its "Erledigt / verworfen" section is the
  project's memory and keeps growing. If it gets unwieldy, move finished
  entries to a `docs/decisions.md` (or into this directory) and keep `PLAN.md`
  to rules, current state and open ideas. Do not shorten the entries: the
  measurements and rejected alternatives are their value.
- **Duplicated rules in Go and JS** (implied crosses, coop operations, cell
  encoding): keep the cross-references in comments; if a third copy appears,
  consider generating constants from Go into the page data instead.

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
  see `PLAN.md`.
- **Persist parties** (e.g. a file or SQLite). Not needed for a home server
  where a restart ending running parties is acceptable. It would also break
  the "no dependencies" property, and should be the first item if that
  changes.

## Done

- **1. One home for puzzle collections** (November 2026): `internal/collection`
  owns the format of `testdata/hard` and `designs` (index, file names, header
  lines, ID, duplicate checks, create-never-overwrite). The root package keeps
  only the embeds, `web` only the library pages, and the designer and `hard`
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
- **3, step 1: `Logic` into `logic.js`** (November 2026): the pure functions
  are their own asset, tested by `logic_test.js`; `app.js` is only the puzzle
  page's UI. The template slots are now `SHARED` (logic.js, plus app.js on
  the puzzle page) and `SCRIPT` (the page's own). The library, settings and
  designer pages, which only need `Logic`, no longer carry the puzzle page's
  UI: about 67 KB less script each.
