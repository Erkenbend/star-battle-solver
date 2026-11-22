package concviz

// budgetStory: section 11, web.Budget. Two buffered channels as counting
// semaphores, one inside the other: every solve takes a slot, solves
// nobody waits for take a background slot first, of which there is one
// fewer. Each acquire is a select that also watches the caller's context,
// and a solve whose request is gone stops at the solver's next check.
func budgetStory() *Story {
	s := newStory("11-solve-budget.svg", "11. One budget for solving: nested semaphores, a slot kept for pages",
		"internal/web/server.go", 1280, 720)

	lib := []string{"web.Library.start", "web.Budget.Background", "web.Budget.Limit"}
	l1 := s.Go("library 1", Pt{130, 110}, lib...)
	l2 := s.Go("library 2", Pt{130, 290}, lib...)
	l3 := s.Go("library 3", Pt{130, 470}, lib...)
	page := []string{"web.page", "web.Budget.Limit", "solver.cancelled"}
	r1 := s.Go("page: new puzzle", Pt{900, 190}, page...)
	r2 := s.Go("page: /play", Pt{900, 430}, page...)
	v1 := s.Outside("visitor 1", Pt{1170, 70})
	v2 := s.Outside("visitor 2", Pt{1170, 545})

	bg := s.Chan("b.background", "struct{}", 2, Pt{420, 200}, +1, "b.background").Labels('b').Made("web.NewBudget", "max(1, n - 1)")
	slots := s.Chan("b.slots", "struct{}", 3, Pt{640, 310}, +1, "b.slots").Labels('b').Made("web.NewBudget", "n")
	never := s.Chan("ctx.Done()", "struct{}", 0, Pt{420, 500}, -1, "ctx.Done()").Labels('b').Outside("context.Background(): never closed")
	ctx1 := s.Chan("ctx.Done()", "struct{}", 0, Pt{1170, 190}, -1, "ctx.Done()", "done").Labels('b').Outside("request of visitor 1")
	ctx2 := s.Chan("ctx.Done()", "struct{}", 0, Pt{1170, 430}, -1, "ctx.Done()").Labels('b').Outside("request of visitor 2")

	for _, l := range []*G{l1, l2, l3} {
		s.Flow(l, bg)
	}
	s.Flow(bg, slots)
	s.Flow(never, l3)
	s.Flow(r1, slots)
	s.Flow(r2, slots)
	s.Flow(v1, ctx1)
	s.Flow(v2, ctx2)
	s.Flow(ctx1, r1)
	s.Flow(ctx2, r2)

	acquire := func(g *G, c, ctx *C) { g.Select(SendCase(c, "{}"), RecvCase(ctx)) }

	s.Step("A crawler opens hard library puzzles; each page starts its solve in the background (section 8). Nobody waits for these, so each takes a background slot first, then a slot. Both are selects that also watch the solve's context, here one that never ends.")
	l1.Arrive("once.Do: go")
	acquire(l1, bg, never)
	acquire(l1, slots, never)
	l1.Busy("solving")

	s.Step("The second does the same. Now b.background is full: two of the three slots are taken by solves nobody waits for.")
	l2.Arrive("once.Do: go")
	acquire(l2, bg, never)
	acquire(l2, slots, never)
	l2.Busy("solving")

	s.Step("The third blocks on b.background. The rest of the crawl queues up here too, not in front of the pages.")
	l3.Arrive("once.Do: go")
	acquire(l3, bg, never)

	s.Step("A player asks for a new puzzle. A page's solve needs only a slot, and the third is free: it solves at once.")
	r1.Arrive("net/http: new request")
	acquire(r1, slots, ctx1)
	r1.Busy("solving")

	s.Step("A second player pastes a puzzle. All three slots are taken, so its select blocks until a slot frees up or its request ends.")
	r2.Arrive("net/http: new request")
	acquire(r2, slots, ctx2)

	s.Step("Visitor 2 gives up and closes the tab. The select takes the context's case: the handler returns without having solved anything.")
	v2.Close(ctx2)
	r2.Exit("returned: visitor left")

	s.Step("Visitor 1 leaves too, while its puzzle is solved. Before its next lookahead trial the solver polls Done (solver.cancelled) and ends Cancelled; the deferred receive frees the slot.")
	v1.Close(ctx1)
	r1.Poll(ctx1)
	r1.Recv(slots)
	r1.Exit("returned: visitor left")

	s.Step("Library 1 is done. Its deferred receives free its slot, then its background slot, in reverse order. That releases library 3, which takes a slot next.")
	l1.Recv(slots)
	l1.Recv(bg)
	l1.Exit("returned: result kept")
	acquire(l3, slots, never)
	l3.Busy("solving")

	s.Step("Library 2 and 3 end the same way. The rest of the crawl follows, two at a time, and a page always finds the third slot.")
	for _, l := range []*G{l2, l3} {
		l.Recv(slots)
		l.Recv(bg)
		l.Exit("returned: result kept")
	}

	return s.end()
}
