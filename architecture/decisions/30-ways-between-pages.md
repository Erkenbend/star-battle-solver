# 30. Ways between pages only through the tab bar, one way to the next puzzle

- **Status:** done
- **Date:** 2026-11-21

The puzzle page had grown several ways to other pages and puzzles besides
the site's tab bar: "Neues Puzzle" beside the title (also in the library,
where it gave a solo puzzle), the size of new puzzles under "Mehr" (also in
the library), "Weiter: 10-4" inside the library's "Gelöst" banner, a link
"Alle Einstellungen" under "Mehr", "Zur Lobby" below a party round, and
"Zur Startseite"/"Neue Party" in the party panel once a party was gone.
The user's rule: pages change through the tab bar; on the page, only the
way to the next puzzle, the same in solo and library.

## Decision

- **Next puzzle below the banner, once solved** (`#next-puzzle` in `.news`,
  `nextPuzzle` in `app.js`): "Neues Puzzle" on a solo page (a submit button
  of the form under "Mehr", so it takes the size chosen there), "Als
  nächstes: <name>" in the library (the next unsolved puzzle of its part).
  Shown in both tabs, so also after looking at the solving path. When every
  puzzle of the part is solved, there is no button and the banner says so.
  Party rounds have none: the next round comes through the party, against
  the clock by itself as before.
- **Before solving, a new solo puzzle only under "Mehr"**, beside its size.
  The pill beside the title (`#newgame-go`, [decision 25](25-mobile-first-page.md))
  is gone. This reverses "frequent actions one tap away" from the review of
  25 for this action: a new puzzle before finishing is the exception.
- **The library has neither "Neues Puzzle" nor sizes:** another size is
  Solo's. "Mehr" there holds "Neu beginnen" and the play options.
- **Removed links:** "Alle Einstellungen" under "Mehr" (the tab bar has the
  settings; the tour of the tutorial still points at them), "Zur Lobby"
  (on a round page the tab "Zusammen spielen" now leads to the party's
  lobby, `/party/CODE`, and back to `/party` once the party is gone), the
  party panel's "Zur Startseite"/"Neue Party" and the party page's "Zur
  Startseite" after an error.

Kept, as exceptions to the rule:

- the error page's buttons (`error.html`, e.g. "Zur Bibliothek" for an
  unknown library id): a page the visitor did not mean to reach, which
  says where to go instead;
- the tutorial's end, which goes back to the page the visitor came from
  (part of the tutorial's flow, not a choice of page);
- the library's tiles (its content) and the party page's form to join a
  party by code;
- automatic changes: the redirect of a new visitor to the tutorial, a
  party moving to its next round, the lobby sending a player to the
  running round, the next puzzle against the clock.

## Checks

UI check: `sizes` solves a solo puzzle and takes "Neues Puzzle" below the
banner (a new 8×8, the select's size); `library`/`libraryphone` take "Als
nächstes" after the motif to the next one; `resume` gets its new puzzle
under "Mehr"; `versus` records where "Zusammen spielen" leads in a round
(`/party/CODE`). Every observation lists `next-go`; on the phone `seen`
records that the button is on screen once solved (it is, in all 10 cases).

Afterwards (on request), the UI check keeps the rule: every observation on
a puzzle page lists the links and forms shown outside the tab bar, and any
but `#next-go` and the form "Neues Puzzle" (`ALLOWED_WAYS` in `lib.mjs`) is
an error of the scenario ("LINKS BESIDE THE TAB BAR"). With `#next-go`
taken out of the allowed ones, `library` and `libraryphone` report both
"Als nächstes" links; the removed links ("Weiter" in the banner, "Alle
Einstellungen" with "Mehr" open, "Zur Lobby") were `a[href]` shown in
observed states and would have been reported the same way.

## Alternatives

- **The button inside the banner,** as "Weiter" was: a small link among
  the banner's text, easy to miss; below it, it is a full-width button
  like the action bar.
- **Keep "Neues Puzzle" beside the title in solo:** two ways to the same
  thing, and the only control above the board.
