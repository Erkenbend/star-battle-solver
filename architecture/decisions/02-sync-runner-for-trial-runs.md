# 2. Trial runs of the lookahead use the sync runner

- **Status:** done
- **Date:** 2026-11-04 (`3bae2e1`)

Trial runs inside the lookahead ("what if a star were here?") always use
`RunSync`, so the chosen explanation is reproducible. The concurrent runner
reaches the same fixpoint, but the order of its steps, and with it the
length of a chain, may differ from run to run.

For the same reason `starbattle html` and `serve` use the sync runner by
default (see [11](11-html-page-and-serve.md)). How the lookahead picks among
refuted assumptions (shortest chain) is described in `PLAN.md`, section
"Suche".
