# Rules

## Orientation

* [architecture/README.md](architecture/README.md): what the parts are, the principles they rely on (section 2), where state lives and where to start for typical changes (section 8). Read it before any non-trivial change.
* [PLAN.md](PLAN.md) and [architecture/decisions](architecture/decisions/README.md): rules, measurements and rejected ideas. Read the matching entries before changing a rule, the search or the generator.
* [docs/development.md](docs/development.md): build, test, UI check and the dev tools `design` and `hardsearch`.
* [grober_plan.md](grober_plan.md) is the only human-written text: never change it.

## Language

* Code in English (identifiers, file names, comments, scripts and their output, data ids, `architecture/`)
* Commits in English (older history is German)
* User interface, solver explanations and user docs (`README.md`, `docs/`, `PLAN.md`) in German
* Chat in the language of the user's prompt

## Coding

* Solo development: commit directly on main unless instructed otherwise
* Commit after each meaningful step (e.g. decision documented, rule implemented, UI feature done), don't batch unrelated steps into one commit
* Push only after the user confirms
* Report choices made autonomously, assume the user is a developer
* Keep the principles of [architecture/README.md](architecture/README.md) section 2, e.g. no third-party dependencies and no JS build step
* Test at behaviour level: prefer a slightly slower but stable integration test to a fast but brittle unit test
* Measure changes to rules, search or generator (on `testdata/hard`, generated puzzles) and record the numbers in a new decision, see [Adding a decision](architecture/decisions/README.md#adding-a-decision). Elsewhere, benchmark only when a problem is suspected or upon request, no scientific-grade precision needed
* `testdata/hard` and `designs` are long-lived data: index files are append-only, see [architecture/README.md](architecture/README.md) sections 4 and 5

## Testing

* Locally run only `go test -short ./...` plus focused `-run` tests; the full suite is CI's job. `STARBATTLE_ALL_HARD=1` runs nowhere by default, only on request (see [decision 22](architecture/decisions/22-ci-sample-only.md))
* After changing browser code, run the UI check (see [docs/development.md](docs/development.md#ui-prüfung)); an intended UI change updates `expected.jsonl` in the same commit
* Moving, renaming or removing an element id: grep `docs/screenshots/shots.mjs` too, and run `docs/screenshots/take.sh` when the pictures show the change
* Look at layout changes in screenshots at phone and desktop size; the UI check sees texts and a few positions (cut-off words in the action bar, what lies under the tab bar), not how a page looks
* Screenshots show states a real game reaches (e.g. stars with the crosses they imply)
* Reproduce a reported bug before fixing it (in the browser: a scratch script with the UI check's helpers); the fix adds an observation to the UI check or a test that fails with the old code
* Text that comes with a tap on the board (banner, tutorial card) goes below it, never above it: the page should feel like a mobile game, not a web page, so the board stays put under the next tap and the text is on screen without scrolling back up (some scrolling is fine, e.g. the action bar moving down; see [decision 28](architecture/decisions/28-news-below-the-board.md)). The UI check records on the phone whether it is on screen (`seen`)
* Browser code must hold up on a phone, on a slow connection and while other players' states keep arriving; the UI check covers this with `PHONE_SCENARIOS`, `underFire` and its errors in every scenario (aborted page loads, the board moving under a tap)

## Documentation

* Update the docs a change affects in the same commit: the table in [architecture/README.md](architecture/README.md) section 8 says which
* After changing goroutines or channels, `go test ./internal/concviz` checks only the story's claims about the code, not the text around it: reread the bullets of that section of [architecture/concurrency.md](architecture/concurrency.md) (and its overview, idioms and testing rows), and grep `architecture/` and `docs/` for the old function and type names
* Refactoring proposals go into [architecture/refactorings.md](architecture/refactorings.md), following the conventions at its top

## Learnings

* When a session brings new insight (a bug fixed, an edge case found, a rule or check that was missing), propose without being asked how the tests or these instructions could catch it next time

## Miscellaneous

* Do not run `find /`, because in WSL it walks the whole filesystem including the Windows drives.
