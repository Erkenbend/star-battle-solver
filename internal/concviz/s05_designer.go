package concviz

// designerStory: section 5, the designer's search (designer.search.run and
// the handler of /api/complete). Workers that loop until a budget cancels
// their context, an unbuffered results channel with a waiter that closes
// it, a handler that selects between results and a ticker, and a mutex for
// the shared set of the k hardest, which also decides whether the expensive
// rating runs at all.
func designerStory() *Story {
	s := newStory("05-designer-stream.svg", "5. The designer's search: workers until a budget, results and progress in one select",
		"internal/designer/designer.go", 1280, 720)

	worker := []string{"designer.search.run", "designer.search.work", "designer.search.fresh", "designer.search.rate", "designer.search.mayEnter", "designer.search.enter"}
	handler := s.Main("handler", Pt{150, 250}, "designer.Handler", "designer.search.run")
	w1 := s.Go("worker 1", Pt{880, 130}, worker...)
	w2 := s.Go("worker 2", Pt{880, 370}, worker...)
	waiter := s.Go("waiter", Pt{500, 470}, "designer.search.run")
	ticker := s.Outside("time.Ticker", Pt{150, 620})
	ctxG := s.Outside("context", Pt{1150, 620})

	out := s.Chan("out", "*Candidate", 0, Pt{500, 250}, -1, "out").Made("designer.search.run", "")
	tick := s.Chan("tick.C", "time.Time", 1, Pt{150, 470}, -1, "tick.C").Labels('l').Outside("time.NewTicker(500 ms)")
	done := s.Chan("ctx.Done()", "struct{}", 0, Pt{1150, 490}, -1, "ctx.Done()").Labels('a').Outside("context.WithCancel(r.Context())")
	wg := s.WaitGroup(Pt{880, 560}, "wg")
	mu := s.Mutex(Pt{1150, 130}, "s.mu")
	best := s.Box("s.best (k = 2)", Pt{1150, 290}, 190, 64, "–")
	page := s.Box("page (NDJSON lines)", Pt{500, 620}, 220, 64, "–")

	s.Flow(w1, out)
	s.Flow(w2, out)
	s.Flow(out, handler)
	s.Flow(tick, handler)
	s.Flow(ticker, tick)
	s.Flow(handler, page)
	s.Flow(waiter, out)
	s.Flow(ctxG, done)
	s.Flow(done, w1)
	s.Flow(done, w2)

	wait := func() { handler.Select(RecvCase(out), RecvCase(tick)) }

	s.Step("The page asks for at most 2 proposals. run derives a context it can cancel from the request's, makes out unbuffered and starts one worker per core but one, 2 here.")
	wg.Go(handler, w1)
	wg.Go(handler, w2)
	w1.Calls("generator.Complete")
	w1.Busy("Complete(seed 4711)")
	w2.Busy("Complete(seed 815)")

	s.Step("A waiter will close out after wg.Wait. The handler waits in a select: a candidate on out, or the ticker.")
	handler.Spawn(waiter)
	waiter.Wait(wg)
	wait()

	s.Step("Every 500 ms the ticker fires. The handler writes the counters, atomics the workers count up, and selects again.")
	ticker.Send(tick, "tick")
	handler.Busy("writes the counters")
	page.Set("progress")
	wait()

	s.Step("Worker 1 has a puzzle, A. Under s.mu it checks that A is new. The rules solve A; best has room, so A enters, again under the lock.")
	w1.Lock(mu)
	w1.Unlock(mu)
	w1.Busy("rules: A solved")
	w1.Lock(mu)
	best.Set("A: 0 free")
	w1.Unlock(mu)

	s.Step("It sends A on out, a handoff to the waiting handler, which writes a line. Worker 1 goes on with a new seed.")
	w1.Send(out, "A")
	handler.Busy("writes A, Flush")
	page.Set("progress, A")
	w1.Busy("Complete(seed 99)")
	wait()

	s.Step("Worker 2's puzzle B gets the rules stuck at 40 free cells. More than A, the last of best (a look under the lock): it may enter, so the solver searches, outside the lock.")
	w2.Busy("rules: B stuck at 40")
	w2.Lock(mu)
	w2.Unlock(mu)
	w2.Calls("s.solve")
	w2.Busy("search: rates B")

	s.Step("Worker 1's next puzzle C: the rules stop at 12 free cells. Also more than A, so it searches too.")
	w1.Busy("rules: C stuck at 12")
	w1.Lock(mu)
	w1.Unlock(mu)
	w1.Busy("search: rates C")

	s.Step("B is rated. It enters best and goes out to the handler.")
	w2.Lock(mu)
	best.Set("B: 40 free", "A: 0 free")
	w2.Unlock(mu)
	w2.Send(out, "B")
	handler.Busy("writes B, Flush")
	page.Set("progress, A, B")
	w2.Busy("Complete(seed 23)")

	s.Step("C enters too and pushes A out of best. The handler is still writing B: worker 1 waits on the unbuffered out, for one line at most.")
	w1.Lock(mu)
	best.Set("B: 40 free", "C: 12 free")
	w1.Unlock(mu)
	w1.Send(out, "C")

	s.Step("The handler takes C. The page sorts what it got as harder does and keeps the first 2: B and C, as in best, in whatever order they came.")
	wait()
	handler.Busy("writes C, Flush")
	page.Set("progress, B, C", "(A dropped)")
	w1.Busy("Complete(seed 3)")
	wait()

	s.Step("Worker 2's D: the rules stop at 5 free cells, fewer than C's 12. D cannot enter, so the costly search never runs for it.")
	w2.Busy("rules: D stuck at 5")
	w2.Lock(mu)
	w2.Unlock(mu)
	w2.Busy("D dropped, no search;\nComplete(seed 77)")

	s.Step("The attempt that uses up the budget, one of worker 1's, calls cancel: ctx.Done() is closed. Closing the page would do the same, through r.Context().")
	w1.Calls("cancel")
	ctxG.Close(done)
	w1.Busy("loop sees ctx.Err()")
	w2.Busy("generator sees Done,\ngives up")

	s.Step("Both loops end, the workers return and wg counts down to 0. Puzzles still being rated would have been sent first: the handler receives until out is closed.")
	w1.Exit("returned")
	w2.Exit("returned")

	s.Step("wg.Wait returns: the waiter cancels the context (also when the budget did not) and closes out. That wakes the handler's select.")
	waiter.Calls("cancel")
	waiter.Close(out)
	waiter.Exit("returned")

	s.Step("The handler sees out closed: it writes done with the final counters and returns.")
	handler.Busy("writes done")
	page.Set("progress, B, C", "done: final counters")
	handler.Exit("returned")

	return s.end()
}
