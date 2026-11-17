package concviz

// runnerStory: section 1, solver.RunConcurrent. Two observers of different
// levels; shows fan-in on out, the priority wait, stale marks, the unbounded
// mailbox with its coalescing notify channel, quiescence, and shutdown by
// close(quit) and wg.Wait.
func runnerStory() *Story {
	s := newStory("01-concurrent-runner.svg", "1. Concurrent rule runner: actors, mailboxes, quiescence",
		"internal/solver/concurrent.go", 1280, 640)

	runner := s.Main("runner", Pt{140, 300}, "solver.RunConcurrent", "solver.mailbox.put")
	a := s.Go("observer A (level 1)", Pt{1141, 125}, "solver.runObserver", "solver.mailbox.take")
	b := s.Go("observer B (level 3)", Pt{1141, 475}, "solver.runObserver", "solver.mailbox.take")

	state := s.Box("runner state", Pt{140, 125}, 226, 82, "version 0", "pending: –", "idle: –")
	out := s.Chan("out", "msg", 2, Pt{619, 300}, -1, "out").Made("solver.RunConcurrent", "len(observers)")
	quit := s.Chan("quit", "struct{}", 0, Pt{1170, 300}, +1, "quit").Labels('l').Made("solver.RunConcurrent", "")
	mbA := s.Box("mailbox A (mutex + slice)", Pt{510, 125}, 190, 50, "queue: –")
	mbB := s.Box("mailbox B (mutex + slice)", Pt{510, 475}, 190, 50, "queue: –")
	notA := s.Chan("notify", "struct{}", 1, Pt{800, 125}, +1, "b.notify", "box.notify").Made("solver.newMailbox", "1")
	notB := s.Chan("notify", "struct{}", 1, Pt{800, 475}, +1, "b.notify", "box.notify").Made("solver.newMailbox", "1")
	wg := s.WaitGroup(Pt{140, 475}, "wg")

	s.Flow(a, out)
	s.Flow(b, out)
	s.Flow(out, runner)
	s.Flow(runner, mbA)
	s.Flow(mbA, notA)
	s.Flow(notA, a)
	s.Flow(runner, mbB)
	s.Flow(mbB, notB)
	s.Flow(notB, b)
	s.Flow(quit, a)
	s.Flow(quit, b)

	s.Step("RunConcurrent starts one goroutine per observer with wg.Go. Each works on its own copy of the board, at version 0.")
	wg.Go(runner, a)
	wg.Go(runner, b)
	a.Busy("deduces on v0")
	b.Busy("deduces on v0")

	s.Step("The runner is the only reader of out. It blocks until a message arrives.")
	runner.Recv(out)

	s.Step("B finds marks first and sends them; the runner keeps them pending: A has a lower level and is not idle at v0 yet.")
	b.Select(SendCase(out, "marks@v0"), RecvCase(quit))
	runner.Store(state, "version 0", "pending: B marks@v0", "idle: –")
	runner.Recv(out)
	b.Select(RecvCase(notB), RecvCase(quit))

	s.Step("A finds marks too. The lowest level goes first: the runner applies A's marks, version 1. B's pending marks are stale now and dropped.")
	a.Select(SendCase(out, "marks@v0"), RecvCase(quit))
	runner.Store(state, "version 1", "pending: – (B's dropped)", "idle: –")
	a.Select(RecvCase(notA), RecvCase(quit))

	s.Step("Broadcast: put appends batch v1 to each mailbox and does a non-blocking send on notify. The runner never blocks here.")
	runner.Give(mbA, "boxes[id].put", "v1", "queue: v1")
	runner.TrySend(notA, "wake")
	runner.Give(mbB, "boxes[id].put", "v1", "queue: v1")
	runner.TrySend(notB, "wake")
	runner.Recv(out)

	s.Step("Both observers wake up and take their whole queue. A is quick; B's rule is slow this time.")
	a.Take(mbA, "box.take", "v1", "queue: –")
	b.Take(mbB, "box.take", "v1", "queue: –")
	a.Busy("deduces on v1")
	b.Busy("deduces on v1 (slow)")

	s.Step("A sends new marks. No simpler rule exists, so the runner applies them at once: version 2.")
	a.Select(SendCase(out, "marks@v1"), RecvCase(quit))
	runner.Store(state, "version 2", "pending: –", "idle: –")
	a.Select(RecvCase(notA), RecvCase(quit))

	s.Step("Broadcast v2. A wakes up. B is busy, so its wake-up waits in the 1-slot buffer of notify.")
	runner.Give(mbA, "boxes[id].put", "v2", "queue: v2")
	runner.TrySend(notA, "wake")
	runner.Give(mbB, "boxes[id].put", "v2", "queue: v2")
	runner.TrySend(notB, "wake")
	runner.Recv(out)

	s.Step("A catches up and finds more marks: version 3.")
	a.Take(mbA, "box.take", "v2", "queue: –")
	a.Select(SendCase(out, "marks@v2"), RecvCase(quit))
	runner.Store(state, "version 3", "pending: –", "idle: –")
	a.Select(RecvCase(notA), RecvCase(quit))

	s.Step("Broadcast v3. B's notify slot is still full, so this wake-up is dropped; batch v3 is queued all the same.")
	runner.Give(mbA, "boxes[id].put", "v3", "queue: v3")
	runner.TrySend(notA, "wake")
	runner.Give(mbB, "boxes[id].put", "v3", "queue: v2 v3")
	runner.TrySend(notB, "wake")

	s.Step("Meanwhile B reports marks from v1. The runner is busy, but out has room for one message per observer: B does not block.")
	b.Select(SendCase(out, "marks@v1"), RecvCase(quit))

	s.Step("The runner reads B's message from the buffer. Marks found on v1 are stale at v3 and ignored: B will deduce again.")
	runner.Recv(out)
	runner.Drop("stale: dropped")
	runner.Recv(out)

	s.Step("B's select finds the pending wake-up without blocking. take returns v2 and v3 together: one wake-up was enough.")
	b.Select(RecvCase(notB), RecvCase(quit))
	b.Take(mbB, "box.take", "v2 v3", "queue: –")
	b.Busy("deduces on v3")
	a.Take(mbA, "box.take", "v3", "queue: –")
	a.Busy("deduces on v3")

	s.Step("Neither finds anything new: both report idle at v3 and wait for the next broadcast.")
	a.Select(SendCase(out, "idle@v3"), RecvCase(quit))
	runner.Store(state, "version 3", "pending: –", "idle: A@v3")
	runner.Recv(out)
	b.Select(SendCase(out, "idle@v3"), RecvCase(quit))
	runner.Store(state, "version 3", "pending: –", "idle: A@v3 B@v3")
	a.Select(RecvCase(notA), RecvCase(quit))
	b.Select(RecvCase(notB), RecvCase(quit))
	runner.Busy("all idle at v3: quiescent")

	s.Step("Every observer is idle at the current version, so nothing is in flight. The deferred close(quit) wakes both at once; wg.Wait waits for them.")
	runner.Close(quit)
	runner.Wait(wg)

	s.Step("The observers return; wg counts down to 0 and wg.Wait returns. No goroutine outlives RunConcurrent.")
	a.Exit("returned")
	b.Exit("returned")
	runner.Exit("returns the result")

	return s.end()
}
