# 25. Puzzle page laid out for phones first

**Status:** done with variant B (see "Variant B": the decision below plus
its changes), on main since 2026-11-20. Variant A stays on branch
`mobile-ui-a` for reference, B on `mobile-ui-b`; "Neues Puzzle" beside the
title replaced by [decision 30](30-ways-between-pages.md)  
**Date:** 2026-11-20

## Decision

The puzzle page (`serve` and `html`) is laid out for phones and tablets
first, the desktop gets the same structure with more room. Nothing was
removed; things moved:

- **Always on screen:** the board; below it one short line with the star
  tool "★ Tippen", ⓘ and the clock; the tabs "Selbst lösen | Lösungsweg"
  across the width.
- **Action bar** (`.actionbar`): the buttons of the shown tab, icon above a
  short word. Up to 860 px it is fixed at the bottom, in reach of the thumb;
  above, it sits next to the board as before. Play: Zurück, Vor, Annahme
  (in a hypothesis Widerspruch, Verwerfen), Prüfen, Hinweis, Mehr. Solving
  path: the player buttons and the speed.
- **"Mehr"** (`#more-sheet`): a sheet from the bottom on phones, a popover
  below the bar on wide screens, with what is used now and then: Neues Puzzle
  with the size (and "Mehrere Größen"), Neu beginnen / Alles löschen,
  Aufgeben in a party, and the play options as switches. Served pages showed
  the options only as a line of text with a link to `/einstellungen`; now the
  switches are the same on every page, plus the link (the star symbol stays
  on `/einstellungen` only).
- **ⓘ** opens rules, controls and the legend below the board. Before, the
  rules and controls were a card that was always shown and the legend a line
  below the board.
- **Summary:** the solver's result ("gelöst in 13 Schritten", Annahmen,
  geraten) moved into the solving path tab (`#solver-sum`); above the board
  only what matters for playing (difficulty, no unique solution, against the
  clock). The size is in the title already.
- **Star tool** (new): "★ Tippen" makes a tap set or remove a star, as a
  right click does; dragging still crosses. Not kept across page loads.
  Double tap stays the default.
- Hint shown: the hint button stays pressed (`aria-pressed`) instead of
  changing its label to "Hinweis ausblenden", which did not fit in the bar.

After the first review:

- **"Neues Puzzle" in one tap** beside the title (`#newgame-go`, a submit
  button of the form in "Mehr", so it takes the size chosen there). On
  phones the title drops "Star Battle" to fit beside it in one line.
- **No button stands out:** the hint button is no longer tinted; a button
  that is on (hint shown, playing) is filled (`aria-pressed`).
- **The player** in the solving path has the action bar's look on phones:
  Anfang, Zurück, Abspielen/Pause, Weiter, Ende, and the speed as a select
  under a gauge icon. Before, its buttons and the select ran over the edges
  of the screen.
- **The other pages** (library, settings, party) got the same parts: the
  intro text behind ⓘ beside the title, options as switches, sizes as
  chips, section headings in small capitals. Settings: the play options in
  one list card with one line of explanation each (shortened, the full
  text is in `docs/playing.md`), the symbols as tiles with preview and name
  (two a row on phones), what kind of symbol the chosen one is below them.
  Library: the tabs across the width, three tiles a row on phones, a
  one-line warning on the hard part (the long one behind ⓘ). Party: the
  three modes as one segmented choice with the chosen mode's description
  below; the explanation of lives only once lives are limited.

The tutorial's steps say "Tippen"/"Doppeltippen", and its tour points at the
star tool, Annahme, Mehr and ⓘ in addition to the navigation.

Element ids stayed, so `round.js`, `coop.js`, `race.js`, the tour and the UI
check still find their buttons. The UI check opens "Mehr" before using what
is in it (`more()`), closes it with a tap beside it before tapping the board
again, and on the phone records for every observation whether the action bar
is on screen (variant A).

## Variant B

B keeps everything of A except where things sit on phones and tablets:

- The **action bar** is not fixed: it follows right below the board's line
  (star tool, ⓘ, clock), as a rounded card.
- **"Mehr"** opens in place below the bar instead of as a sheet over the
  page; it never covers the board, and the board takes taps while it is
  open. Closed with another tap on "Mehr".
- The **site navigation** becomes a tab bar fixed at the bottom of every
  page (puzzle, party, library, settings, error), icon above word: Solo,
  Zusammen, Bibliothek, Anleitung, Einstellungen, as in an app. On wide
  screens it stays at the top as before.
- The UI check records on the phone whether the action bar lies under the
  tab bar, instead of whether it is on screen.

Trade-off: A keeps the play buttons always in reach of the thumb and the
navigation small at the top; B makes the navigation app-like and keeps the
play buttons next to the board, so on a tall board or with a hint card they
can scroll away. B is the closer step towards an installable web app.

After the second review (B only):

- **Aufgeben in the bar:** in a party it takes the place of "Vor" (second
  in the bar), against the clock as "Überspringen" with a skip icon and the
  skips left as a badge on it ("Überspringen (2 übrig)" does not fit). In
  coop, where giving up is a vote, the button stays pressed while the vote
  stands instead of changing its word to "Doch nicht aufgeben". "Mehr"
  keeps only Neu beginnen / Alles löschen there.
- **No "Vor" button** anywhere: redo is rarely needed and stays on Ctrl+Y
  (also within a hypothesis). The UI check redoes with Ctrl+Y.
- **No tabs with only one of them:** while "Lösungsweg" is hidden (a party
  round in progress, a hard library puzzle before it is solved), the lone
  filled "Selbst lösen" looked like a big button; the tab row is hidden
  then (`.tabs:has(#tab-watch[hidden])`) and appears once there is a
  solving path. The UI check records whether the tab row shows.
  Changed in [decision 28](28-news-below-the-board.md): the tab is greyed
  out instead, so the row does not push the board down on the solving tap;
  hidden only against the clock.

## Measurements

Solo page `/?n=8&seed=42`, phone 393×760 CSS px, at load:

| | before (main) | A | B |
|---|---|---|---|
| top of the board | 232 px | 135 px | 100 px (103 px after the reviews) |
| page height | 1104 px | 760 px (no scrolling) | 760 px (no scrolling) |
| visible words | 148 | 43 | 44 (43 after the reviews) |
| rows of buttons | 4 | 2 (tabs, bar) | 2 (tabs, bar), plus the tab bar |

## Found on the way

`keepBoard` cleared the min-height it holds on a shrinking card before
measuring it, so a card that shrank twice let the board move up by the
difference when the page could not scroll (the tutorial on a phone, once the
page got shorter). Fixed in its own commit; the phone tutorial scenario of
the UI check caught it.

Variant B, after the first review: the room kept below the pages for the
tab bar did not apply, because each page's own `padding` rule comes later
in `style.css` and won; the end of the library, settings and party pages
lay under the tab bar. Only the screenshots showed it: the UI check ran
these pages at desktop size. Now its phone scenario `pages` scrolls them to
their end and records whether it lies above the tab bar (it reports
"under the tab bar" with the old rule).

Words cut off in the action bar ("Aufgeb…", "Widerspr…") also showed only
in screenshots. The UI check now records them on the phone; at once it found
"Annahme" cut off in a party's six buttons, squeezed by the extra width
given to "Aufgeben". Instead of a width per button, each button now starts
from its word's width (`flex-basis: auto`) and shares the rest of the bar.

## Alternatives

- **Variant A** (branch `mobile-ui-a`): the action bar fixed at the bottom
  of the screen, "Mehr" as a sheet over the page, the site navigation at
  the top. The review chose B: app-like navigation in reach of the thumb,
  nothing lying over the board.
- Hiding the site navigation behind a menu button: the navigation fits in
  one line on a 393 px phone and is not what made the page feel crowded.
- A help sheet instead of the inline ⓘ card: an overlay hides the board the
  legend talks about.

Open: with a web-app manifest the page could later be installed. Done in
[decision 26](26-installable-web-app.md).
