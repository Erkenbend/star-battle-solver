# 24. Animations for `concurrency.md`, checked against the code

- **Status:** partly done (sections 1, 4 and 8 of 10)
- **Date:** 2026-11-17

[`concurrency.md`](../concurrency.md) is meant as a real-world example of
goroutines and channels for developers. Prose explains a pattern's shape, but
not well who blocks on what, when, and who wakes whom. So each section gets an
animation of one run: goroutines as boxes coloured by state (running, blocked,
returned), channels with their buffer slots, values moving between them,
dashed arrows from a blocked goroutine to what it waits on, and a caption per
step.

How it is built (`internal/concviz`, standard library only):

- **A story per section**, written in Go: a scene and a script of operations
  (`Send`, `Recv`, `Range`, `Select`, `TrySend`, `Poll`, `Close`, `WG.Go`,
  `Wait`, `Once.Do`, …). The script runs on a small model of Go's semantics:
  buffers and handoffs, select with and without default, close waking every
  receiver, WaitGroup counters. Blocking and waking follow from the
  operations, so a story cannot show a send on a full buffer as non-blocking;
  a send on a closed channel, a blocked goroutine acting, or a goroutine left
  running at the end panics.
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
- **Animated SVG with CSS keyframes**, one opacity animation per state of an
  element and a transform animation per value, looping. GitHub shows such
  SVGs in Markdown, they stay crisp at any size, and the colours follow
  `prefers-color-scheme`. Like the sprite sheets, the files are checked in and
  `TestAnimationsUpToDate` compares them with what the stories draw;
  `go test ./internal/concviz -update` rewrites them.

Numbers: the runner story has 16 steps and loops every 89 s (54 KB), the
pipeline 13 steps, 75 s (38 KB), the library future 8 steps, 44 s (18 KB).
Step time is 1.1 s of movement plus 1.3 s and 0.03 s per caption character
to read. All tests of the package take 0.02 s.

Alternatives considered:

- **Recording real runs.** A trace hook (nil by default) at each of the ten
  places, tests that run the real code and assert the orderings the
  animation shows (e.g. "results are emitted in seed order"). It would catch
  a change of behaviour that keeps the structure, which the claims do not.
  Not done for now: it touches production code everywhere for the sake of
  documentation, and real interleavings vary between runs, so the animation
  could not be drawn from the recording anyway.
- **GIF with `image/gif`.** Animated in any viewer, but the standard library
  has no font rendering, the files would be larger, and they could not
  follow the dark theme.
- **SMIL** (`<animate>`) instead of CSS: would work as well; one CSS
  keyframe list per element was simpler to generate.

Open: animations for sections 2, 3, 5, 6, 7, 9 and 10. The model may need a
few additions, e.g. a mutex for section 7, a ticker for section 9, and
results written by index for section 2.
