// Package concviz draws the animations of architecture/concurrency.md: one
// animated SVG per pattern, showing goroutines, channels, and who waits on
// what.
//
// Each animation is a story: a scene (goroutines, channels, boxes for other
// state) and a script of steps. The script runs on a small model of Go's
// synchronisation semantics (channel buffers and handoffs, select with and
// without default, close as a broadcast, sync.WaitGroup, sync.Once), so who
// blocks and who wakes up follows from the operations instead of being drawn
// by hand. A story that sends on a closed channel, or lets a blocked
// goroutine act, panics.
//
// Every operation also names the code it stands for: a goroutine lists the
// functions its code is in, a channel where it is made and with which
// capacity, and each send, receive, select, close or call becomes a claim
// about those functions. TestStoriesMatchCode checks every claim against the
// source with go/ast, so renaming a channel, changing a capacity or dropping
// a select case breaks the test until the story follows.
//
// The SVGs are checked in under architecture/concurrency/. After changing a
// story or the drawing, rewrite them with
//
//	go test ./internal/concviz -update
package concviz

// Stories builds all animations, in the order of architecture/concurrency.md.
func Stories() []*Story {
	return []*Story{runnerStory(), hardsearchStory(), libraryStory()}
}
