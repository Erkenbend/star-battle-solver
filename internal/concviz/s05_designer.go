package concviz

// designerStory: section 5, the designer's search (designer.search.run and
// the handler of /api/complete). Workers that loop until a context is
// cancelled (by the k-th puzzle, the budget of attempts or a gone request),
// an unbuffered results channel with a waiter that closes it, a handler
// that selects between results and a ticker, and a mutex for the shared
// seeds and puzzles seen.
func designerStory() *Story {
	s := newStory("05-designer-stream.svg", "5. The designer's search: workers until k puzzles, results and progress in one select",
		"internal/designer/designer.go", 1280, 720)

	worker := []string{"designer.search.run", "designer.search.work", "designer.search.newSeed", "designer.search.accept"}
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
	seen := s.Box("s.seen (k = 2)", Pt{1150, 290}, 190, 64, "–")
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

	s.Step("The page asks for 2 proposals. run derives a context it can cancel from the request's, makes out unbuffered and starts one worker per core but one, 2 here.")
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

	s.Step("Worker 1 has a puzzle, A. Under s.mu, accept finds it new and the first of 2. The solver rates it outside the lock.")
	w1.Lock(mu)
	seen.Set("A")
	w1.Unlock(mu)
	w1.Calls("s.solve")
	w1.Busy("solver rates A")

	s.Step("It sends A on out, a handoff to the waiting handler, which writes a line. Worker 1 goes on with a new seed.")
	w1.Send(out, "A")
	handler.Busy("writes A, Flush")
	page.Set("progress, A")
	w1.Busy("Complete(seed 99)")
	wait()

	s.Step("Worker 2's seed gave A as well. accept counts a duplicate, and worker 2 draws the next seed, also under the lock.")
	w2.Lock(mu)
	w2.Unlock(mu)
	w2.Busy("duplicate;\nComplete(seed 23)")

	s.Step("Worker 2 finds B, the second of 2. accept calls s.cancel(): ctx.Done() is closed. The budget of attempts and a closed page end the search the same way.")
	w2.Lock(mu)
	seen.Set("A, B")
	w2.Calls("s.cancel")
	ctxG.Close(done)
	w2.Unlock(mu)
	w2.Busy("solver rates B")

	s.Step("Worker 1's running attempt polls ctx.Done() and gives up. Its loop sees ctx.Err() and ends; worker 1 returns.")
	w1.Busy("generator sees Done,\ngives up")
	w1.Exit("returned")

	s.Step("B is rated and sent, a handoff again. Worker 2's loop ends too: wg counts down to 0. The handler receives until out is closed, so no send is left waiting.")
	w2.Send(out, "B")
	handler.Busy("writes B, Flush")
	page.Set("progress, A, B")
	w2.Exit("returned")

	s.Step("wg.Wait returns: the waiter cancels the context (also when nothing did yet) and closes out.")
	wait()
	waiter.Calls("s.cancel")
	waiter.Close(out)
	waiter.Exit("returned")

	s.Step("The handler's select sees out closed: it writes done with the final counters and returns.")
	handler.Busy("writes done")
	page.Set("progress, A, B", "done: final counters")
	handler.Exit("returned")

	return s.end()
}
