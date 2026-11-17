package concviz

// hardsearchStory: section 4, hardsearch.Search. Two workers; shows the
// handoffs of unbuffered channels, the reorder buffer, back-pressure,
// cancellation through ctx.Done() and the closer goroutine that ends the
// caller's range.
func hardsearchStory() *Story {
	s := newStory("04-hardsearch-pipeline.svg", "4. Hard puzzle search: ordered pipeline, cancellation",
		"internal/hardsearch/hardsearch.go", 1280, 600)

	caller := s.Main("caller", Pt{1150, 210}, "hardsearch.Search")
	prod := s.Go("producer", Pt{130, 210}, "hardsearch.Search")
	w1 := s.Go("worker 1", Pt{643, 105}, "hardsearch.Search", "hardsearch.check")
	w2 := s.Go("worker 2", Pt{643, 315}, "hardsearch.Search", "hardsearch.check")
	closer := s.Go("closer", Pt{873, 440}, "hardsearch.Search")
	sig := s.Outside("Ctrl-C", Pt{130, 440})

	jobs := s.Chan("jobs", "uint64", 0, Pt{387, 210}, +1, "jobs").Made("hardsearch.Search", "")
	results := s.Chan("results", "Found", 0, Pt{913, 210}, +1, "results").Made("hardsearch.Search", "")
	done := s.Chan("ctx.Done()", "struct{}", 0, Pt{387, 440}, -1, "ctx.Done()").Labels('b').Outside("closed by cancel()")
	wg := s.WaitGroup(Pt{582, 440}, "wg")
	pending := s.Box("reorder buffer", Pt{1170, 345}, 180, 50, "next 10, pending –")
	emitted := s.Box("emit, in seed order", Pt{1170, 440}, 180, 50, "–")

	s.Flow(prod, jobs)
	s.Flow(jobs, w1)
	s.Flow(jobs, w2)
	s.Flow(w1, results)
	s.Flow(w2, results)
	s.Flow(results, caller)
	s.Flow(caller, pending)
	s.Flow(sig, done)
	s.Flow(done, prod)
	s.Flow(closer, results)

	s.Step("Search makes two unbuffered channels and starts two workers with wg.Go. Each ranges over jobs and blocks: nothing is sent yet.")
	wg.Go(caller, w1)
	wg.Go(caller, w2)
	w1.Range(jobs)
	w2.Range(jobs)

	s.Step("It starts a producer and a closer, which waits for the workers. The caller itself ranges over results.")
	caller.Spawn(prod)
	caller.Spawn(closer)
	closer.Wait(wg)
	caller.Range(results)

	s.Step("Unbuffered means every send is a handoff to a waiting receiver: seed 10 goes to worker 1, seed 11 to worker 2.")
	prod.Select(SendCase(jobs, "seed 10"), RecvCase(done))
	prod.Select(SendCase(jobs, "seed 11"), RecvCase(done))
	w1.Calls("check", "Generate")
	w1.Busy("check(seed 10)")
	w2.Busy("check(seed 11)")

	s.Step("No worker is free, so the producer blocks in its select until one takes seed 12 or the context is cancelled.")
	prod.Select(SendCase(jobs, "seed 12"), RecvCase(done))

	s.Step("Worker 2 finishes first. Seed 11 is out of order: the caller keeps it in the reorder buffer.")
	w2.Send(results, "res 11")
	caller.Store(pending, "next 10, pending 11")
	caller.Range(results)

	s.Step("Worker 2 asks for the next seed: the producer's waiting send completes. It offers seed 13 and blocks again.")
	w2.Range(jobs)
	w2.Busy("check(seed 12)")
	prod.Select(SendCase(jobs, "seed 13"), RecvCase(done))

	s.Step("Worker 1 delivers seed 10. Now 10 and 11 are in sequence: the caller emits both, from one goroutine, and writes their files.")
	w1.Send(results, "res 10")
	caller.Store(pending, "next 12, pending –")
	caller.Calls("emit")
	emitted.Set("10, 11")
	caller.Busy("emit: writes files")
	w1.Range(jobs)
	w1.Busy("check(seed 13)")
	prod.Select(SendCase(jobs, "seed 14"), RecvCase(done))

	s.Step("Worker 2 finishes seed 12 while the caller is still writing. Its send must wait: back-pressure through the unbuffered channel.")
	w2.Send(results, "res 12")

	s.Step("Ctrl-C cancels the context: ctx.Done() is closed. The producer's select wakes up and returns; its deferred close(jobs) follows.")
	sig.Close(done)
	prod.Close(jobs)
	prod.Exit("returned")
	w1.Busy("generator sees Done,\naborts seed 13")

	s.Step("The caller takes seed 12 and emits it. Worker 2's range over the closed jobs ends: it returns, wg counts down.")
	caller.Range(results)
	caller.Store(pending, "next 13, pending –")
	emitted.Set("10, 11, 12")
	w2.Range(jobs)
	w2.Exit("returned")
	caller.Range(results)

	s.Step("Seed 13 comes back with an error because it was cancelled. The caller drops it: a gap where the run ends.")
	w1.Send(results, "res 13 ✗")
	caller.Drop("cancelled: dropped")
	caller.Range(results)
	w1.Range(jobs)
	w1.Exit("returned")

	s.Step("The last worker is gone, so wg.Wait returns and the closer closes results. That ends the caller's range loop.")
	closer.Close(results)
	closer.Exit("returned")

	s.Step("Search returns 13, the first seed not emitted: -from 13 continues the run without holes.")
	caller.Exit("returns 13")

	return s.end()
}
