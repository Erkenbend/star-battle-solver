package concviz

// libraryStory: section 8, web.Library.start. A lazy future: sync.Once
// starts the solver, close(e.ready) publishes its result to any number of
// waiters, done() peeks without blocking, and a waiter can give up through
// its request's context.
func libraryStory() *Story {
	s := newStory("08-library-future.svg", "8. Lazy solving of library puzzles: a future from sync.Once and close",
		"internal/web/library.go", 1280, 720)

	pageFns := []string{"web.libraryRoutes", "web.Library.data", "web.libEntry.done", "web.Library.start"}
	weg := []string{"web.libraryRoutes", "web.Library.start"}
	p1 := s.Go("page #1", Pt{130, 400}, pageFns...)
	w1 := s.Go("/weg #1", Pt{470, 400}, weg...)
	w2 := s.Go("/weg #2", Pt{810, 400}, weg...)
	p3 := s.Go("page #3", Pt{1150, 400}, pageFns...)
	solver := s.Go("solver", Pt{640, 110}, "web.Library.start")
	tab := s.Outside("visitor 1", Pt{130, 545})

	once := s.Once(Pt{150, 110}, "e.once")
	fields := s.Box("e.sol, e.res, e.err", Pt{1108, 110}, 190, 44, "not set")
	ready := s.Chan("e.ready", "struct{}", 0, Pt{640, 255}, +1, "e.ready").Labels('r').Made("web.loadLibrary", "")
	ctx1 := s.Chan("r.Context().Done()", "struct{}", 0, Pt{470, 545}, -1, "r.Context().Done()").Labels('b').Outside("request of visitor 1")
	ctx2 := s.Chan("r.Context().Done()", "struct{}", 0, Pt{810, 545}, -1, "r.Context().Done()").Labels('b').Outside("request of visitor 2")

	s.Flow(solver, fields)
	s.Flow(solver, ready)
	s.Flow(ready, p1)
	s.Flow(ready, w1)
	s.Flow(ready, w2)
	s.Flow(ready, p3)
	s.Flow(tab, ctx1)
	s.Flow(ctx1, w1)
	s.Flow(ctx2, w2)

	s.Step("Visitor 1 opens a library puzzle (page #1). Building the page, done() peeks at e.ready with a select with default: it never blocks.")
	p1.Arrive("net/http: new request")
	p1.Calls("lib.data")
	p1.Poll(ready)

	s.Step("Then lib.start calls once.Do. The first call starts the solver goroutine, whose defer close(e.ready) is the promise.")
	p1.Calls("lib.start")
	p1.Do(once, func() { p1.Spawn(solver) })
	solver.Calls("bruteforce.Solve", "lib.solve")
	solver.Busy("bruteforce, then solve")
	p1.Exit("page sent, no trace yet")

	s.Step("Later the page asks /bibliothek/weg for the trace (/weg #1). once.Do does nothing now; the handler waits on e.ready or its request.")
	w1.Arrive("net/http: new request")
	w1.Calls("lib.start")
	w1.Do(once, func() { w1.Spawn(solver) })
	w1.Select(RecvCase(ready), RecvCase(ctx1))

	s.Step("A second visitor asks too (/weg #2). Any number of goroutines can wait on the same channel.")
	w2.Arrive("net/http: new request")
	w2.Do(once, func() { w2.Spawn(solver) })
	w2.Select(RecvCase(ready), RecvCase(ctx2))

	s.Step("Visitor 1 closes the tab, which cancels their request: that handler returns. The solver goes on for whoever comes next.")
	tab.Close(ctx1)
	w1.Exit("returned: visitor left")

	s.Step("The solver sets sol, res and err, then its deferred close(e.ready) wakes every waiter at once.")
	fields.Set("set")
	solver.Close(ready)
	solver.Exit("returned")

	s.Step("What was written before the close is safe to read after the receive: no lock needed for e.res.")
	w2.Busy("reads e.res, sends JSON")
	w2.Exit("returned")

	s.Step("A closed channel stays ready forever: for visitor 3 (page #3), done() is true at once and the page carries the trace.")
	p3.Arrive("net/http: new request")
	p3.Poll(ready)
	p3.Do(once, func() { p3.Spawn(solver) })
	p3.Exit("page sent with trace")

	return s.end()
}
