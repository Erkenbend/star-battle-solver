# 14. Playing together (party mode)

- **Status:** done
- **Date:** 2026-11-07 (`818a6f8` to `46ab527`)

Implemented under `/party` (navigation "Solo | Zusammen spielen" on all pages
of `serve`; solo stays the start page). This entry describes the first mode,
playing against each other; "together on one board" (2026-11-08, `914ce5c`)
and "against the clock" (2026-11-14, `80d8660`) came later and are described
in [`docs/playing.md`](../../docs/playing.md).

- **Flow:** create a party with the allowed sizes → a code of 4 letters
  (without I/O), link `/party/ABCD`. Join with a name; the next round starts
  as soon as everyone present has clicked "Bereit" (3 s countdown, the board
  is hidden until then). Each round takes a random one of the allowed sizes.
  The round ends when all present participants have solved or given up.
- **Ranking per round:** fewest hints, then shortest time; the server
  measures the time from the common start (it also runs while the tab is
  hidden). Hints and "Fehler zeigen" cost as in solo, "Prüfen" is free. The
  solving path only after solving or giving up. Giving up shows the solution
  and ranks behind everyone who solved.
- **Server (`party.Manager`, today in `internal/party`):** everything in
  memory behind one mutex; the next puzzle is prepared in the background when
  a round starts (from the pool first, otherwise generated). The server
  checks submitted solutions, hints only count upwards. Absent is whoever has
  had no event stream open for 60 s; absent players block neither the end of
  a round nor the start. Whoever joins during the countdown plays along,
  later arrivals from the next round on. Parties expire after 2 h without
  activity (at most 50 parties, 16 people).
- **Transport (`party_server.go`):** state as server-sent events
  (`/party/{code}/events`), actions as JSON POST (`join`, `ready`,
  `progress`, `solve`, `giveup`, `leave`); a token per party in
  `localStorage`, reloading leads back into the round. Pages: `party.html`
  (create/join/lobby) and the round page `/party/{code}/round/{n}` (the
  puzzle page with `Data.Party`), both with `party.js`.
- Tests: state machine with a fake clock, HTTP and SSE in `internal/web`; the
  UI was played through with jsdom against a running server (two players,
  hint, solving, giving up, reloading, second round); that script was not
  kept in the repo (since replaced by the UI check in `internal/web/uicheck`).

Open: an overall ranking across all rounds; the others' progress only as a
number of stars.
