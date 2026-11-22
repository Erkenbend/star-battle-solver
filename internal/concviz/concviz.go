// Package concviz draws the diagrams of architecture/concurrency.md: for
// each pattern, one run as a sequence diagram (inline in the Markdown) and as
// an animated scene in architecture/concurrency/player.html, which plays it
// step by step.
//
// Each run is a story: a scene (goroutines, channels, boxes for other
// state) and a script of steps. The script runs on a small model of Go's
// synchronisation semantics (channel buffers and handoffs, select with and
// without default, close as a broadcast, sync.WaitGroup, sync.Once,
// sync.Mutex), so who blocks and who wakes up follows from the operations
// instead of being drawn by hand. A story that sends on a closed channel, or
// lets a blocked goroutine act, panics.
//
// Every operation also names the code it stands for: a goroutine lists the
// functions its code is in, a channel where it is made (in a function or a
// package-level var) and with which capacity, and each send, receive,
// select, close, call or write by index becomes a claim about those
// functions. TestStoriesMatchCode checks every claim against the
// source with go/ast, so renaming a channel, changing a capacity or dropping
// a select case breaks the test until the story follows.
//
// The diagrams, the player and each story's block in concurrency.md (between
// <!-- concviz … --> lines) are generated and checked in. After changing a
// story or the drawing, rewrite them with
//
//	go test ./internal/concviz -update
package concviz

// Stories builds all stories, in the order of architecture/concurrency.md.
func Stories() []*Story {
	return []*Story{runnerStory(), lookaheadStory(), generatorStory(), hardsearchStory(), designerStory(), poolStory(), prepStory(), libraryStory(), eventsStory(), lifecycleStory(), budgetStory()}
}
