# 19. README badges and test coverage

- **Status:** done
- **Date:** 2026-11-14 (`7423374`)

Badges at the top of the README: CI, coverage, library (number of puzzles),
platforms, Go version, no dependencies, licence, vibe-coded. Coverage and
library size are computed by the job `badges` in `ci.yml` on pushes to `main`
and stored as shields.io JSON on the branch `badges` (without history, no
external service). Checked afterwards: the job runs, the branch has both
files, the badges show up in the README.

Rejected: a release badge, Codecov/Go Report Card, stars/forks, a live demo
(IPv6 only and with a login).

Coverage across all packages (`-coverpkg=./...`), locally with `-short` in
about 20 s: before 84 %, now 96.1 %. The biggest gap was `cmd/starbattle`
(0 %); new smoke tests call the commands directly (stdin/stdout through
temporary files, `serve`/`design` with an invalid address so they return
immediately). Plus error paths in `web` (designer messages, `saveDesign`,
`LoadLibrary`, parties), `board`, `puzzle`, `render` and `generator`. What
remains is mainly `main`/`usage` (they end the process), the shutdown of
`serve`/`design` and rare error paths in `web`.
