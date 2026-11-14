# 11. HTML page and `serve`

- **Status:** done
- **Date:** 2026-11-05 to 2026-11-07 (`917b251`, `324be50`, `bddbc03` and
  following)

Implemented as `starbattle html [-o page.html] [solver options] puzzle.txt`
(default runner `sync`, so the explanation is reproducible, see
[2](02-sync-runner-for-trial-runs.md)).

- **Solving yourself:** a click cycles free → ✕ → ★, right click/shift sets ★
  directly; undo/redo; neighbours of a star optionally crossed automatically
  (off by default; the last choice applies per device to new puzzles and
  party rounds, puzzles with saved progress keep their own); rule violations
  shown live in red (two stars, touching, a unit without room). "Prüfen"
  says for free whether everything so far matches the solution; "Fehler
  zeigen" marks wrong cells red and costs one hint per wrong cell. Progress in
  `localStorage`, plus a clock and the number of hints shown (the clock only
  runs while the page is visible and stops when solved; the solved board is
  frozen – no input, touch scrolls the page; "Gelöst in 3:42 mit 2
  Hinweisen"; "Neu beginnen" resets both). After solving, "Ergebnis kopieren"
  copies e.g. "Star Battle 8×8 in 3:42 gelöst (mit 2 Hinweisen)!
  http://…/?n=8&seed=123" to the clipboard (link only with `serve`; without
  HTTPS via `execCommand`, because `navigator.clipboard` needs a secure
  context).
- **Hint:** the first step of the trace that still sets something new on the
  board. All steps before it are already there, so its preconditions hold
  (the rules are monotone). "Anwenden" applies it, "Hinweis ausblenden" hides
  it again; with wrong cells on the board, "Fehler zeigen" first.
- **Solving path:** step by step (buttons, ←/→, playback with speed), a label
  per rule coloured by level; reasoning from assumptions indented and played
  along ("Gedankenexperiment", contradiction cells red), each can be hidden.
  Rows/columns/colours not named in the explanation are dimmed (the page reads
  "Zeilen 3 bis 5", "Farben B und C", "(2,3)" from the text).
- **Dragging** sets many ✕ at once (started on a ✕: removes them), stars stay
  untouched, one drag = one undo step; also by touch.
- **Colours instead of letters:** all texts show a swatch in the region's
  colour instead of "Farbe B"; pointing at it (touch: tapping) highlights the
  region on the board. A fixed palette of 12 matched colours (more regions:
  computed hues), with own variants for dark mode. The letters in the cell
  corners are gone.
- Light/dark following the system setting, phone layout without horizontal
  scrolling.
- Tests: `internal/web` (data, escaping, `logic_test.js` with node: whoever
  only follows the hints solves all hard puzzles without mistakes). The UI
  itself was checked with jsdom and Chromium screenshots; the README
  screenshots are taken by `docs/screenshots/take.sh`, the jsdom checks were
  not kept in the repo (since replaced by the UI check in
  `internal/web/uicheck`).
- **`starbattle serve [-addr localhost:6369] [solver options]`:** the same
  page from a server, with "Neues Puzzle" (size 4 to 14). Every puzzle has a
  fixed URL (`/?n=8&seed=123` like `generate -seed`, `/play?p=AAB/…`), so
  reloading keeps puzzle and progress. Ambiguous puzzles are shown without a
  solution, unsolvable ones rejected.
- **Pool (`web.Pool`):** `serve` keeps one generated and solved puzzle ready
  for each size 4 to 14; a background goroutine fills the sizes one after
  another (small ones first) and refills after every hand-out. "Neues Puzzle"
  (`/?n=N`) redirects to the seed of the ready puzzle, whose page then comes
  without computing (14×14: 2.5 ms instead of 2.6 to 3.9 s). The puzzle is
  exactly the one of its seed, so the URL stays valid. If the size is empty
  at the moment, a random seed is made as before. `-no-pregen` switches it
  off.

Open: hints on boards that deviate from the trace are not always the simplest
possible step (feasible now with `serve`: start the solver on the current
board); a star by long press on the phone.
