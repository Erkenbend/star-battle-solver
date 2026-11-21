# 31. "/party" leads back to the party one plays in

- **Status:** done
- **Date:** 2026-11-21

Follows [decision 30](30-ways-between-pages.md): with "Zur Lobby" gone, the
tab "Zusammen spielen" is the way to a party's lobby. On a round page it
pointed there already, but from any other page (Solo, Bibliothek, the
settings) it led to `/party`, the page to create a new party, and the party
one was in could only be found again through its link or code. "/" leads
back to the unsolved solo puzzle ([decision 27](27-solo-resumes.md)); "/party"
now does the same for the party, while leaving one on purpose, e.g. to
create a new one, stays possible.

## Decision

- **Cookie `starbattle.party`** holding `CODE.TOKEN`. The party's pages
  (lobby and rounds, `rememberParty` in `party.js`) set it whenever a state
  shows this browser as a player, and remove it when a state does not, the
  party is gone or the player leaves, but only if it holds this party:
  looking at another party's lobby keeps one's own. Seven days; an idle
  party is removed after two hours anyway, and the server checks.
- **The server checks it** (`memberOf` in `party_server.go`): `/party`
  redirects to `/party/CODE` only if that party exists and the token is
  one of its players. So a stale cookie (party expired, server restarted,
  player removed) gives the page to create or join, without the page
  having to clean up first.
- **The token in the cookie,** not only the code: without it the server
  could only check that the party exists, and a player who left on another
  way would be sent to a lobby they are no longer in, again and again. The
  token is a per-player secret of a friendly game; it already travels in
  every POST and in the event stream's URL.
- **Leaving:** "Party verlassen" in the lobby as before, and new under "Mehr"
  on a round page (`#party-leave`): the lobby sends a player of the running
  round straight back to it, so during a round the lobby's button was out
  of reach. Both go through `Party.leave`: confirm, leave on the server,
  forget token and cookie, then `/party`. No other page change follows it
  (`goTo` is shut), so a state arriving meanwhile cannot send the player
  back into the round.
- `fresh()` of the UI check removes the cookie too and starts on
  `/einstellungen` instead of `/party`, which with a leftover cookie would
  open a lobby (and its event stream).

## Checks

- `TestPartyCookie`: a member goes to the lobby; no cookie, an empty or
  wrong token, another code, a lower-case code, no separator, garbage, and
  the same cookie after `Leave` give the party page.
- UI check, `versus`: during the second party's running round `/party`
  ends on the round's page (`/party/CODE/round/1`, through the lobby);
  "Party verlassen" under "Mehr" leads to `/party` with the form to create
  one, and `/party` stays there afterwards. Every observation lists
  `party-leave`: shown in the rounds of all four party scenarios, hidden
  elsewhere.

## Alternatives

- **Local storage and a script on `/party`:** the page would load the
  create form first and then move on, two loads and a flash of the wrong
  page; the same reason as for "/" in decision 27.
- **Only the code in the cookie, the page cleans up:** see the token above.
- **A link "Neue Party" on the lobby next to "Party verlassen":** two
  memberships at once, and a way to another page beside the tab bar.
