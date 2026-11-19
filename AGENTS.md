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

## Documentation

* Update the docs a change affects in the same commit: the table in [architecture/README.md](architecture/README.md) section 8 says which
* Refactoring proposals go into [architecture/refactorings.md](architecture/refactorings.md), following the conventions at its top

## Miscellaneous

* Do not run `find /`, because in WSL it walks the whole filesystem including the Windows drives.
