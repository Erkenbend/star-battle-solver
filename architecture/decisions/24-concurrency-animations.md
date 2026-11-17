# 24. Diagrams and a step-through player for `concurrency.md`, checked against the code

- **Status:** partly done (sections 1, 4 and 8 of 10)
- **Date:** 2026-11-17

[`concurrency.md`](../concurrency.md) is meant as a real-world example of
goroutines and channels for developers. Prose explains a pattern's shape, but
not well who blocks on what, when, and who wakes whom. So each section shows
one run of its pattern, in two forms drawn from the same story:

- inline, a **sequence diagram**: a column per goroutine, channel and other
  shared state, time downwards, one band per numbered step. A running
  goroutine is a solid line, a blocked one a dashed orange line labelled with
  what it waits on; values are arrows, a value waiting in a buffer is a bar
  on the channel, a closed channel turns red, a WaitGroup's counter shows
  where it changes (orange while `Wait` would block, green at 0). The
  numbered steps follow as a list in the Markdown;
- in [`concurrency/player.html`](../concurrency/player.html), opened locally
  in a browser, the **animated scene**: goroutines as boxes coloured by state,
  channels with their buffer slots, values moving between them, dashed
  arrows from a blocked goroutine to what it waits on. It plays, pauses,
  steps forward and back (buttons or arrow keys), scrubs, and lists the steps
  with what each goroutine did in them. The list scrolls on its own, so
  stepping never moves the page; the last step ends with the timeline.

How it is built (`internal/concviz`, standard library only):

- **A story per section**, written in Go: a scene and a script of operations
  (`Send`, `Recv`, `Range`, `Select`, `TrySend`, `Poll`, `Close`, `WG.Go`,
  `Wait`, `Once.Do`, …). The script runs on a small model of Go's semantics:
  buffers and handoffs, select with and without default, close waking every
  receiver, WaitGroup counters. Blocking and waking follow from the
  operations, so a story cannot show a send on a full buffer as non-blocking;
  a send on a closed channel, a blocked goroutine acting, or a goroutine left
  running at the end panics. The model records the scene after each step
  (for the player) and every event with the goroutines' states after it (for
  the diagram).
- **Claims about the code.** Each goroutine lists the functions its code is
  in, each channel where it is made, with which element type and capacity.
  Every operation then becomes a claim, e.g. "`solver.runObserver` has
  `select { out <- …; <-quit }`" or "`hardsearch.Search` makes `jobs` as
  `chan uint64` with no capacity". `TestStoriesMatchCode` checks all of them
  with `go/ast` (16 claims per story at the time of writing). Changing the
  capacity of `out` or renaming `jobs` was tried on purpose: the test names
  the claims that broke, e.g. "makes out as chan msg with capacity
  len(observers): not found, but it is made as make(chan msg, 2 *
  len(observers))".
- **Generated files, checked in**: the diagrams (`concurrency/NN-*.svg`), the
  player, and the block of each story in `concurrency.md` (between
  `<!-- concviz … -->` lines: diagram, link, steps). Like the sprite sheets,
  `TestOutputsUpToDate` compares them with what the stories draw, and
  `go test ./internal/concviz -update` rewrites them.
- **The player** is one self-contained HTML file without dependencies. Each
  scene is an SVG animated with CSS keyframes (one opacity animation per
  state of an element, a transform animation per value); the page puts it
  into a shadow root and drives all its animations through the Web
  Animations API (`getAnimations`, `currentTime`, `pause`), so seeking and
  stepping need no second renderer in JavaScript.

Numbers: the runner story has 16 steps (scene 81 s at normal speed), the
pipeline 13 steps (68 s), the library future 8 steps (38 s). The diagrams are
8 to 16 KB and 900 to 980 px wide, the player 154 KB. All tests of the
package take 0.02 s.

Tried first and replaced (same day): the looping animated SVGs alone, inline
in `concurrency.md`. GitHub shows SVGs in Markdown as images, without
scripts or interaction, so they could be neither paused nor rewound; with 8
to 16 steps of one to three sentences each, that was too much to follow.

Alternatives considered:

- **A static storyboard** (one still frame per step) inline: readable at
  one's own pace, but 16 frames of the scene take far more room than one
  diagram, and the diagram shows the waiting over time better.
- **Shorter clips** of 3 to 5 steps each: still not controllable.
- **Recording real runs.** A trace hook (nil by default) at each of the ten
  places, tests that run the real code and assert the orderings the diagram
  shows (e.g. "results are emitted in seed order"). It would catch a change
  of behaviour that keeps the structure, which the claims do not. Not done
  for now: it touches production code everywhere for the sake of
  documentation, and real interleavings vary between runs, so the diagram
  could not be drawn from the recording anyway.
- **GIF with `image/gif`.** Animated in any viewer, but the standard library
  has no font rendering, the files would be larger, and they could not
  follow the dark theme.

Open:

- Diagrams for sections 2, 3, 5, 6, 7, 9 and 10. The model may need a few
  additions, e.g. a mutex for section 7, a ticker for section 9, and results
  written by index for section 2.
- Publishing the player (GitHub Pages) so that the link in `concurrency.md`
  opens it on GitHub too; for now it is opened from a clone.
