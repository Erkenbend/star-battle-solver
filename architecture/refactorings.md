# Architecture review and suggested refactorings

Review as of November 2026 (about 90 commits, ~18k lines including tests and
assets). None of the refactorings below have been carried out yet. When one is
done, remove it here and update [README.md](README.md) if the structure
changed. When one is rejected, note why, so it is not proposed again.

## Verdict

The core is in good shape and needs no restructuring: `puzzle` → `board` →
`solver` → `observers` is layered cleanly, dependencies only point downwards,
rules are pure and tested against an independent oracle, and the two runners
share all their semantics. The generator, brute force and hard search are
small, focused packages.

The pressure is at the edges, where features have accumulated:

- `internal/web` holds over 40 % of all non-test Go code and mixes six
  concerns (page rendering, site routes, library, parties, symbols, the
  developer designer).
- `assets/app.js` (~2,150 lines) is one file whose `boot()` builds the UI for
  every kind of page.
- The format of the puzzle *collections* (index files, header lines such as
  `# rating:` and `# name:`) is read and written in three different places.

The suggestions are ordered by value for effort. None is urgent; each is worth
doing when the area is touched next anyway.

## 1. One home for puzzle collections (recommended)

**Now:** a "collection" is a directory of puzzle files plus `bibliothek.idx`,
with header lines in each file. The knowledge about it is spread out:

- the root package (`hard.go`) reads index files and the `# rating:` line;
- `cmd/starbattle` writes the header of new hard puzzles and rewrites rating
  lines (`writeHard`, `rateDir`, `withRating`);
- `internal/web/designer.go` writes new motif files, their `# name:` line, and
  appends to the index; `internal/web/library.go` derives motif names.

**Suggestion:** one package (e.g. `internal/collection`, or the root package
if it is renamed, see 4) that owns the collection format: read and append to
an index, read and set header lines, find duplicates, name new files. `hard`,
the designer and the library then call it instead of each knowing part of the
format.

**Why:** these files are long-lived data, and the append-only rule of the
index is the one invariant whose violation silently breaks players' saved
progress. It should be enforced in one place. It also takes domain logic out
of `main.go`, which should stay a thin dispatcher.

**Effort:** small to medium; behaviour unchanged, existing tests cover it.

## 2. Split `internal/web` along its concerns (recommended, when next touching parties)

**Now:** one package of ~3,400 lines (without tests). The party engine
(`party.go`, `coop.go`, `clock.go`, ~1,500 lines) is a state machine that
does not need HTTP, but it shares the package with routes and page rendering
and hands out finished page data (`RoundData` builds a `web.Data`).

**Suggestion:**

- `internal/party`: parties, rounds, the three modes, ranking; no HTTP. It
  returns puzzles, solutions and solver results, and `web` turns them into
  pages. Its HTTP/SSE layer (`party_server.go`) stays in `web`.
- `internal/designer` (or `internal/web/designer`): the developer tool, which
  is not part of `serve` anyway. It needs the shared page helpers, which would
  then be exported from `web` (or moved into a small `internal/web/page`).
- `web` keeps rendering, site routes, library, pool, symbols, tutorial.

**Why:** the boundary "game rules vs. transport" already exists in the file
split and the comments; making it a package boundary keeps it from eroding
and makes the party tests independent of page rendering. The designer split
makes visible that it is not shipped as part of the site.

**Effort:** medium. Mostly moving code and deciding which names to export.
Do it as a pure move, without behaviour changes in the same commit.

## 3. Split `app.js` into files (recommended for `Logic`, optional beyond)

**Now:** `app.js` contains the pure `Logic` module (~550 lines, tested with
node) and `boot()` (~1,600 lines in one closure) covering solo play,
playback of the trace, hints, the hypothesis mode, library, the three party
modes and the tutorial.

**Suggestion, step 1 (easy):** move `Logic` into its own asset `logic.js`,
inlined before `app.js` by the existing template mechanism and loaded
directly by `logic_test.js`. This makes the testable part and the DOM part
visibly separate and keeps new pure logic from landing in `boot()`.

**Step 2 (only if the file keeps growing):** cut mode-specific UI (party
modes, tutorial, hypothesis mode) out of `boot()` into separate files that
plug into a small, explicit interface of the page (board redraw, lock,
storage key, toolbar). This is harder than it looks because the parts share
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
