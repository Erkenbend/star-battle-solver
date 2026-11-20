# 27. "/" leads back to the unsolved solo puzzle

- **Status:** done
- **Date:** 2026-11-20

Takes up an open point of [decision 26](26-installable-web-app.md). "Solo"
in the navigation, the start page and the start of the installed app all
lead to `/`, which always gave a new puzzle. In a browser the back button
gets you back to your puzzle; an installed app on iOS has no back button.
So going from a puzzle to the settings and back lost your place: the
progress was still in local storage, under a URL nobody knew any more.

## Decision

- `/` without a query redirects to the **last solo puzzle opened or played,
  while it is unsolved**, with its marks, time and hints as saved.
  "Neues Puzzle" sends `/?n=…`, so it still gives a new one, and once that
  puzzle is solved `/` gives a new one too.
- **Where:** a cookie `starbattle.solo` holding the puzzle's URL. The
  puzzle page sets it on every save (`rememberSolo` in `app.js`) unless it
  is already there, and removes it once solved, but only if it still holds
  this page's URL (another tab may have put its own puzzle there since).
  The server reads it in the handler of `/` (`soloURL` in `server.go`).
  It lasts 90 days, `SameSite=Lax`.
- **Only solo puzzles:** `/?n=…&seed=…` and `/play?p=…`. Library puzzles
  have their own way back (Bibliothek) and their own "solved" list, party
  rounds belong to the party, and the tutorial keeps nothing.
- **Validation:** the server accepts only these two forms, with a size
  within the limits and a puzzle that parses, and rebuilds the URL from
  their parts. Anything else in the cookie is ignored (a new puzzle, as
  before), so it cannot redirect to another site.

## Checks

- `TestSoloCookie`: the cookie's puzzle is where `/` goes; parameters in
  another order are rebuilt; other hosts, `//host`, a size out of range, a
  bad seed, a repeated parameter, a puzzle that does not parse, other paths
  and garbage give a new puzzle; `/?n=7` ignores the cookie.
- UI check, phone scenario `resume`: a cross on `/?n=6&seed=3`, then
  "Solo" from the settings and `/` itself land there with the cross.
  "Neues Puzzle" gives a new one, and "Solo" from the library goes back to
  that one. Solved, "Solo" gives a new puzzle. With the old code "Solo"
  and `/` gave new random puzzles (`/?n=8&seed=25242`, empty board).
  `fresh()` removes the cookie, so a scenario's leftover puzzle does not
  meet the next scenario at `/`.

## Alternatives

- **Local storage and links rewritten by the page:** the page would set
  the href of every "/" link to the puzzle. The error page has no script,
  the installed app's start URL is fixed in the manifest, and so is
  typing the site's address: all of them would still get a new puzzle. The
  cookie works on all of them, because the server answers `/`.
- **Script on `/` that moves on to the puzzle:** `/` redirects on the
  server before any script runs, so this would load a new puzzle first
  and then the old one. Two page loads, and a flash of the wrong board.
- **Back to the last puzzle even when solved:** the solved board is
  frozen; a new puzzle is what "Solo" is for then.
- **Any puzzle with marks instead of the last one opened:** "Neues Puzzle",
  then the settings and back would return to the older puzzle rather than
  the one just opened.

## Found on the way

The `resume` scenario's solve on the phone reported "BOARD MOVED": on a
solo page shorter than the screen (6×6 or 8×8 on a phone) the "Gelöst"
banner pushed the board down by 96 px on the solving tap. `keepBoard` gave
the page room below by exactly the shift, which is enough only at the end
of a page longer than the screen. Fixed in its own commit (`c09b609`), with
a phone scenario `short` that solves a 6×6.
