package concviz

// generatorStory: section 3, generator.run. Two buffered channels as
// counting semaphores, one per call and one for the process; the lowest
// successful attempt wins through an atomic minimum, so the result does not
// depend on the number of workers.
func generatorStory() *Story {
	s := newStory("03-generator-attempts.svg", "3. Parallel generator attempts: semaphores, a deterministic winner",
		"internal/generator/generator.go", 1280, 680)

	run := s.Main("run", Pt{140, 280}, "generator.run", "generator.cancelled")
	a0 := s.Go("attempt 0", Pt{900, 150}, "generator.run")
	a1 := s.Go("attempt 1", Pt{900, 370}, "generator.run")
	pool := s.Outside("another Generate call", Pt{540, 520})

	slots := s.Chan("slots", "struct{}", 2, Pt{480, 190}, +1, "slots").Labels('r').Made("generator.run", "workers")
	cpu := s.Chan("cpuSlots", "struct{}", 3, Pt{480, 370}, +1, "cpuSlots").Labels('b').
		Made("generator.cpuSlots", "max(1, runtime.GOMAXPROCS(0) - 1)")
	done := s.Chan("o.Done", "struct{}", 0, Pt{140, 420}, +1, "done", "o.Done").Labels('b').Outside("the caller's ctx.Done(), or nil")
	wg := s.WaitGroup(Pt{290, 530}, "wg")
	best := s.Box("best (atomic.Int64)", Pt{1150, 480}, 200, 50, "1000: none yet")
	results := s.Box("results: by attempt", Pt{1150, 260}, 200, 64, "[0] –", "[1] –")

	s.Flow(run, slots)
	s.Flow(run, cpu)
	s.Flow(pool, cpu)
	s.Flow(done, run)
	s.Flow(a0, results)
	s.Flow(a1, results)

	s.Step("run with 2 workers: slots (capacity 2) limits this call, cpuSlots (GOMAXPROCS−1, 3 here) every call in the process together. Another call, the server's pool, holds one already.")
	pool.Send(cpu, "{}")

	s.Step("Attempt 0: acquire is a send into each buffer. Then the loop checks best and Done (a select with default), and starts the attempt with its own random generator.")
	run.Send(slots, "{}")
	run.Send(cpu, "{}")
	run.Calls("best.Load")
	run.Poll(done)
	wg.Go(run, a0)
	a0.Busy("attempt(seed 0)")

	s.Step("Attempt 1 the same way. Now both buffers are full: three attempts run in the process, two of them from this call.")
	run.Send(slots, "{}")
	run.Send(cpu, "{}")
	run.Poll(done)
	wg.Go(run, a1)
	a1.Busy("attempt(seed 1)")

	s.Step("For attempt 2 the send on slots blocks: a buffered channel is a counting semaphore, and its buffer is full.")
	run.Send(slots, "{}")

	s.Step("Attempt 1 succeeds first. It writes results[1] and lowers best from 1000 to 1 with a compare-and-swap loop.")
	a1.Write(results, "results[i]", "puzzle", "[0] –", "[1] puzzle")
	a1.Calls("best.CompareAndSwap")
	best.Set("1")

	s.Step("Its deferred release receives from cpuSlots, then from slots. That frees the loop: its waiting value moves into the buffer of slots.")
	a1.Recv(cpu)
	a1.Recv(slots)
	a1.Exit("returned")

	s.Step("The loop acquires cpuSlots too, but 2 > best: attempt 2 would lose anyway and is not started. It releases both and waits for attempt 0.")
	run.Send(cpu, "{}")
	run.Calls("best.Load")
	run.Recv(cpu)
	run.Recv(slots)
	run.Wait(wg)

	s.Step("Attempt 0 goes on: its index is below best, so it can still win. It succeeds, writes results[0] and lowers best to 0.")
	a0.Write(results, "results[i]", "puzzle", "[0] puzzle", "[1] puzzle")
	best.Set("0")
	a0.Recv(cpu)
	a0.Recv(slots)
	a0.Exit("returned")

	s.Step("run returns results[0], the lowest successful attempt: the same puzzle as with one worker. More workers only make it arrive sooner.")
	run.Exit("returns results[0]")

	return s.end()
}
