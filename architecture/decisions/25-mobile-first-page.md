# 25. Puzzle page laid out for phones first

**Status:** proposed, two variants for review: A on branch `mobile-ui-a`, B
(this text) on branch `mobile-ui-b`, built on A; see "Variant B" below
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

## Measurements

Solo page `/?n=8&seed=42`, phone 393×760 CSS px, at load:

| | before (main) | A | B |
|---|---|---|---|
| top of the board | 232 px | 135 px | 100 px |
| page height | 1104 px | 760 px (no scrolling) | 760 px (no scrolling) |
| visible words | 148 | 43 | 44 |
| rows of buttons | 4 | 2 (tabs, bar) | 2 (tabs, bar), plus the tab bar |

## Found on the way

`keepBoard` cleared the min-height it holds on a shrinking card before
measuring it, so a card that shrank twice let the board move up by the
difference when the page could not scroll (the tutorial on a phone, once the
page got shorter). Fixed in its own commit; the phone tutorial scenario of
the UI check caught it.

## Alternatives

- **Variant B**, see above.
- Hiding the site navigation behind a menu button: the navigation fits in
  one line on a 393 px phone and is not what made the page feel crowded.
- A help sheet instead of the inline ⓘ card: an overlay hides the board the
  legend talks about.

Open: the party, library and settings pages keep their layout; with a
web-app manifest the page could later be installed.
