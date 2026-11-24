# Rules

## Orientation

* [architecture/README.md](architecture/README.md): what the parts are, the principles they rely on (section 2), where state lives and where to start for typical changes (section 8). Read it before any non-trivial change.
* [PLAN.md](PLAN.md) and [architecture/decisions](architecture/decisions/README.md): rules, measurements and rejected ideas. Read the matching entries before changing a rule, the search or the generator.
* [docs/development.md](docs/development.md): build, test, UI check and the dev tools `design`, `hardsearch` and `stress`.
* [grober_plan.md](grober_plan.md) is the only human-written text: never change it.

## Language

* Code in English (identifiers, file names, comments, scripts and their output, data ids, `architecture/`)
* Commits in English (older history is German)
* User interface, solver explanations and user docs (`README.md`, `docs/`, `PLAN.md`) in German
* Chat in the language of the user's prompt

## Coding

* Solo development: commit directly on main unless instructed otherwise
* Go code is formatted with `gofmt -s` (CI checks it); the pre-commit hook in `.githooks` does it, enable it in a new clone with `git config core.hooksPath .githooks`
* Commit after each meaningful step (e.g. decision documented, rule implemented, UI feature done), don't batch unrelated steps into one commit
* Push only after the user confirms
* Report choices made autonomously, assume the user is a developer
* What differs between kinds of puzzle page (tutorial, solo, file, motif, hard, party modes) goes into `Logic.pageKind` and its node test, not into new flags combined in `app.js`
* Keep the principles of [architecture/README.md](architecture/README.md) section 2, e.g. no third-party dependencies and no JS build step
* Test at behaviour level: prefer a slightly slower but stable integration test to a fast but brittle unit test
* Measure changes to rules, search or generator (on `testdata/hard`, generated puzzles) and record the numbers in a new decision, see [Adding a decision](architecture/decisions/README.md#adding-a-decision). Elsewhere, benchmark only when a problem is suspected or upon request, no scientific-grade precision needed
* Changes to how `serve` schedules its work (solve budget, pool, cache, library, party preparation) or to what it costs to serve a page (compression, page size, wrappers around the handler): measure under load with `starbattle stress` before and after (`-server` with a build of the previous commit, `-serve-args` for an option of `serve`; add a scenario if none covers it), not with a load test in the test process, which mixes the client's CPU into the server's; look at the latency of a page that waits, not only at throughput; one semaphore that looked right starved the pages (decision 33)
* `testdata/hard` and `designs` are long-lived data: index files are append-only, see [architecture/README.md](architecture/README.md) sections 4 and 5

## Testing

* Locally run only `go test -short ./...` plus focused `-run` tests; the full suite is CI's job. `STARBATTLE_ALL_HARD=1` runs nowhere by default, only on request (see [decision 22](architecture/decisions/22-ci-sample-only.md))
* After changing browser code, run the UI check's groups of the pages changed (see [docs/development.md](docs/development.md#ui-prüfung)), all of them only after changing what every page uses (`logic.js`, `style.css`, the site's tab bar); an intended UI change updates the group's file in `expected/` in the same commit
* Moving, renaming or removing an element id: grep `docs/screenshots/shots.mjs` too, and run `docs/screenshots/take.sh` when the pictures show the change
* A change to rules, search or `board` keeps `internal/web/testdata/traces` as it is unless it is meant to change explanations; then `go test ./internal/web -run TestTraceGolden -update` rewrites them in the same commit, and their diff is checked like code
* Look at layout changes in screenshots at phone and desktop size; the UI check sees texts and a few positions (cut-off words in the action bar, what lies under the tab bar), not how a page looks
* Screenshots show states a real game reaches (e.g. stars with the crosses they imply)
* Reproduce a reported bug before fixing it (in the browser: a scratch script with the UI check's helpers); the fix adds an observation to the UI check or a test that fails with the old code
* Text that comes with a tap on the board (banner, tutorial card) goes below it, never above it: the page should feel like a mobile game, not a web page, so the board stays put under the next tap and the text is on screen without scrolling back up (some scrolling is fine, e.g. the action bar moving down; see [decision 28](architecture/decisions/28-news-below-the-board.md)). The UI check records on the phone whether it is on screen (`seen`)
* Browser code must hold up on a phone, on a slow connection and while other players' states keep arriving; the UI check covers this with `PHONE_SCENARIOS`, `underFire` and its errors in every scenario (aborted page loads, the board moving under a tap)

## Documentation

* Update the docs a change affects in the same commit: the table in [architecture/README.md](architecture/README.md) section 8 says which
* After changing goroutines or channels, `go test ./internal/concviz` checks only the story's claims about the code, not the text around it: reread the bullets of that section of [architecture/concurrency.md](architecture/concurrency.md) (and its overview, idioms and testing rows), and grep `architecture/` and `docs/` for the old function and type names
* Refactoring proposals go into [architecture/refactorings.md](architecture/refactorings.md), following the conventions at its top
* [HISTORY.md](HISTORY.md) is updated only on request, not with every commit. Then: take the commits after the end commit of its newest section (`git log --reverse --date=format:'%Y-%m-%d %a %H:%M'`, with bodies) and group them into sessions (a morning, afternoon or evening; a gap of a few hours starts a new one; if the first new commits continue the newest section's session, extend that section instead). Each session gets a section on top titled ``YYYY-MM-DD <Vormittag|Mittag|Nachmittag|Abend> ([`start...end`](https://github.com/Erkenbend/star-battle-solver/compare/start~1...end))`` with a few short German bullets on features and technical (non-functional) improvements, not one per commit and nothing about AGENTS.md or other working rules; decisions, refactorings and docs it names are Markdown links. Add a picture where the session changed something visible: a file from `docs/screenshots` as of the end commit (`git show end:docs/screenshots/x.png`), or a new shot of the end commit in a temporary `git worktree`; pictures go to `docs/history/YYYY-MM-DD-<topic>.png`, never into `docs/screenshots`, phone shots as `<img ... width="300">`. Finally set the line `Stand:` to today's date and the last commit

## Learnings

* When a session brings new insight (a bug fixed, an edge case found, a rule or check that was missing), propose without being asked how the tests or these instructions could catch it next time

## Miscellaneous

* Do not run `find /`, because in WSL it walks the whole filesystem including the Windows drives.
