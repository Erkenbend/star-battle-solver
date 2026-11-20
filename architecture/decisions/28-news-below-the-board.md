# 28. Banner and tutorial card below the board

- **Status:** done
- **Date:** 2026-11-20

The puzzle page showed the banner (solved, hypothesis, a party's news, the
solving path's "Gedankenexperiment") and the tutorial's card above the
board. So that the next tap still landed where the player aimed, `keepBoard`
scrolled the page by as much as they grew, and the board stayed where it
was on screen. But then the banner itself lay above the screen: after
solving a library puzzle on a phone, "Gelöst … Weiter: 10-4" had to be
found by scrolling back up. The page felt like a web page, not a game.

## Decision

- **Below the board on phones, beside it on wide screens.** Banner and
  tutorial card sit in their own grid item `.news` between the board and
  the panels: on phones (one column) below the board's line with the star
  tool and clock, above the action bar, which moves down as they come; above
  860 px at the top of the column beside the board, above the action bar.
  There, below the board would be below the screen: the board takes the
  height of the window.
- **`keepBoard` removed.** Nothing above the board changes height on a tap
  any more, so there is nothing to compensate: no scrolling by the page, no
  extra room at its end, no held heights.
- **The tab "Lösungsweg" is greyed out, not hidden,** while the solving
  path comes later (a party round, a hard library puzzle before it is
  solved). Hidden, the tab row appeared on the solving tap and pushed the
  board down by 50 px; before, `keepBoard` had scrolled the header out of
  view and the browser's scroll anchoring hid this. A row of two tabs with
  one greyed out does not look like a lone big button, which is why the row
  was hidden ([decision 25](25-mobile-first-page.md)). Against the clock,
  where the solving path never comes, the tab and its row stay hidden.
- The UI check records, on the phone, whether a shown banner or tutorial
  card is on screen, above it or below it (`seen`), and runs the library
  scenario on the phone too (`libraryphone`).

Found on the way: the party page's `.share` box (`display: flex`) also
matched the banner's "Ergebnis kopieren" button, which therefore took a line
of its own. It is inline now (`18a88fa`).

## Measurements

UI check, phone scenarios (393×851), observations with a banner shown:

| | before | after |
|---|---|---|
| banner on screen | 15 | 28 |
| banner above the screen | 13 | 0 |
| tutorial card on screen | 18 | 18 |
| board moved under a tap | 0 | 0 (50 px three times with the tab row still hidden) |

The 13 were the solved library puzzles (hard and motif), and in party
rounds the hypothesis, solving, giving up, wrong stars with lives and
the coop vote. `app.js` and `tutorial.js` lost 23 lines.

## Alternatives

- **Keep `keepBoard`, scroll the banner into view after it came:** the
  board would move under the next tap, which is what `keepBoard` was for.
- **Below the board on wide screens too:** below the screen at 1200×800,
  the screenshot of a solved library puzzle no longer showed its banner.
- **The banner below the action bar,** next to the hint card: the bar
  would stay put, but the banner would come further down, under the tab
  bar on a large board.
- **Reserve the tab row's height while it is hidden:** a blank gap above
  the board, which looks like a layout bug.

Open: the ⓘ card stays inside the board's part, so when it is open on a
phone the banner comes below it.
