package concviz

// designerStory: section 5, designer.completeDesign. One goroutine per
// candidate, results streamed through a channel with a buffer of exactly k,
// a waiter goroutine that closes it, and a mutex for the shared set of seen
// layouts.
func designerStory() *Story {
	s := newStory("05-designer-stream.svg", "5. Design completions streamed to the page: fan-out, a buffer of k, a waiter",
		"internal/designer/designer.go", 1280, 680)

	handler := s.Main("handler", Pt{140, 250}, "designer.Handler", "designer.completeDesign")
	c1 := s.Go("candidate 1", Pt{900, 110}, "designer.completeDesign")
	c2 := s.Go("candidate 2", Pt{900, 250}, "designer.completeDesign")
	c3 := s.Go("candidate 3", Pt{900, 390}, "designer.completeDesign")
	waiter := s.Go("waiter", Pt{520, 500}, "designer.completeDesign")

	// The handler ranges over the call's result directly.
	ranged := "completeDesign(req.Pattern, req.Strict, k, solve, r.Context().Done())"
	out := s.Chan("out", "*foundCandidate", 3, Pt{520, 250}, -1, "out", ranged).Made("designer.completeDesign", "k")
	wg := s.WaitGroup(Pt{900, 520}, "wg")
	mu := s.Mutex(Pt{1160, 180}, "mu")
	seen := s.Box("seen: layouts so far", Pt{1160, 320}, 190, 50, "–")
	page := s.Box("page (NDJSON lines)", Pt{140, 420}, 200, 64, "–")

	s.Flow(c1, out)
	s.Flow(c2, out)
	s.Flow(c3, out)
	s.Flow(out, handler)
	s.Flow(handler, page)
	s.Flow(waiter, out)

	s.Step("The page asks for 3 completions. completeDesign makes out with a buffer of exactly k = 3 and starts one goroutine per candidate.")
	wg.Go(handler, c1)
	wg.Go(handler, c2)
	wg.Go(handler, c3)
	c1.Calls("generator.Complete")
	c1.Busy("generator.Complete")
	c2.Busy("generator.Complete")
	c3.Busy("generator.Complete")

	s.Step("It also starts a waiter that will close out after wg.Wait, and returns out at once. The handler ranges over it, still in the same request.")
	handler.Spawn(waiter)
	waiter.Wait(wg)
	handler.Range(out)

	s.Step("Candidate 2 finds a puzzle first. Under the mutex it records the layout in seen; the lock only guards the map.")
	c2.Lock(mu)
	seen.Set("layout A")
	c2.Unlock(mu)
	c2.Busy("solve: rate the puzzle")

	s.Step("It sends the result: the handler is waiting, so it is a handoff. The page shows the first candidate while the others still run.")
	c2.Send(out, "cand 2")
	c2.Exit("returned")
	handler.Busy("writes a line, Flush")
	page.Set("cand 2")

	s.Step("Candidates 1 and 3 finish together. 1 takes the lock; 3 must wait for it.")
	c1.Lock(mu)
	c3.Lock(mu)

	s.Step("1 has a new layout and unlocks: 3 gets the lock and finds its layout already in seen, a duplicate.")
	seen.Set("layout A, B")
	c1.Unlock(mu)
	c3.Unlock(mu)

	s.Step("Both send while the handler is still writing. With room for k values, no send can block, not even if the page is gone.")
	c3.Send(out, "dup")
	c3.Exit("returned")
	c1.Calls("solve")
	c1.Send(out, "cand 1")
	c1.Exit("returned")

	s.Step("The last candidate is done, so wg.Wait returns and the waiter closes out. Values already in the buffer stay readable.")
	waiter.Close(out)
	waiter.Exit("returned")

	s.Step("The handler drains the buffer: it counts the duplicate and writes candidate 1. Then range ends on the closed, empty channel.")
	handler.Range(out)
	handler.Busy("duplicate: counted")
	handler.Range(out)
	page.Set("cand 2", "cand 1")
	handler.Range(out)

	s.Step("The handler writes the summary line and returns. Closing the page instead would have cancelled r.Context(), which the generator polls.")
	page.Set("cand 2", "cand 1", "done: 2 found, 1 dup")
	handler.Exit("returned")

	return s.end()
}
