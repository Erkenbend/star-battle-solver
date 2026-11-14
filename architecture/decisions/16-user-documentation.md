# 16. User documentation: README with the commands, later split up

- **Status:** done
- **Date:** 2026-11-07 (`36ebe30`), split on 2026-11-14 (`7ee7f63`)

First step (2026-11-07): `README.md` got the puzzle format, all commands with
their options, and the structure of the code. Running it on the Pi stayed in
`deploy/README.md`, background and measurements in `PLAN.md` (the decisions
now in this directory).

Second step: the README had grown to a good 300 lines, two thirds of them
command reference. Now ~100 lines (idea, screenshots, download, links);
details in `docs/commands.md` (puzzle format, commands, the solver options
once in one place), `docs/playing.md` (library, parties, settings from the
players' point of view), `docs/development.md` (building/testing, structure,
`design`, `hard`) and `symbols/README.md` (kinds of symbols, `clips`,
scripts). CI puts `docs/*.md` into the archives as well.

Rejected for now: an animated GIF at the top, a "Probier's aus" link (the
instance is not public).
