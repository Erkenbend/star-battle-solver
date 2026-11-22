package concviz

// lookaheadStory: section 2, solver.lookahead. A worker pool over an
// unbuffered jobs channel; results are written by index instead of sent, a
// shared atomic minimum stops trials that can no longer win, and the best
// one is picked in index order after wg.Wait.
func lookaheadStory() *Story {
	s := newStory("02-lookahead-pool.svg", "2. Parallel lookahead: a worker pool, results by index",
		"internal/solver/search.go", 1280, 600)

	caller := s.Main("lookahead", Pt{140, 230}, "solver.lookahead")
	w1 := s.Go("worker 1", Pt{780, 110}, "solver.lookahead")
	w2 := s.Go("worker 2", Pt{780, 350}, "solver.lookahead")

	jobs := s.Chan("jobs", "int", 0, Pt{460, 230}, +1, "jobs").Made("solver.lookahead", "")
	wg := s.WaitGroup(Pt{140, 440}, "wg")
	hyps := s.Box("hyps: one slot per candidate", Pt{1130, 230}, 230, 104,
		"[0] c0: –", "[1] c1: –", "[2] c2: –", "[3] c3: –")
	shortest := s.Box("best (atomic.Int32)", Pt{1130, 80}, 230, 50, "none yet")
	pick := s.Box("pick, in index order", Pt{1130, 440}, 230, 50, "–")

	s.Flow(caller, jobs)
	s.Flow(jobs, w1)
	s.Flow(jobs, w2)
	s.Flow(w1, hyps)
	s.Flow(w2, hyps)
	s.Flow(w1, shortest)
	s.Flow(w2, shortest)

	slots := func(c0, c1, c2, c3 string) []string {
		return []string{"[0] c0: " + c0, "[1] c1: " + c1, "[2] c2: " + c2, "[3] c3: " + c3}
	}

	s.Step("The rules are stuck; four free cells c0 to c3 are candidates. lookahead makes hyps with one slot each and an unbuffered jobs channel, and starts GOMAXPROCS workers (two here).")
	wg.Go(caller, w1)
	wg.Go(caller, w2)
	w1.Range(jobs)
	w2.Range(jobs)

	s.Step("It sends the indices one by one. Each send is a handoff: index 0 to worker 1, index 1 to worker 2. Each tries a star on its cell, on its own copy of the board; after every step, runSync reads best.")
	caller.Send(jobs, "0")
	caller.Send(jobs, "1")
	w1.Calls("Apply", "runSync", "best.Load")
	w1.Busy("star on c0, runSync")
	w2.Busy("star on c1, runSync")

	s.Step("Both workers are busy, so the send of index 2 blocks until one of them comes back to its range loop.")
	caller.Send(jobs, "2")

	s.Step("The star on c1 leads to a contradiction in 5 steps. Worker 2 writes the chain into hyps[1]: its own slot, so no lock and no channel. Then it lowers best to 5 with a compare-and-swap loop.")
	w2.Write(hyps, "hyps[i]", "5 steps", slots("–", "5 steps", "–", "–")...)
	w2.Calls("best.CompareAndSwap")
	shortest.Set("5")

	s.Step("Worker 2 takes index 2, which releases the caller. Its send of index 3 blocks again.")
	w2.Range(jobs)
	w2.Busy("star on c2, runSync")
	caller.Send(jobs, "3")

	s.Step("The chain of c0 reaches 6 steps without a contradiction. Longer than best, it can no longer be chosen: runSync gives up and hyps[0] stays nil. Worker 1 takes index 3, and the caller closes jobs and waits.")
	w1.Range(jobs)
	w1.Busy("star on c3, runSync")
	caller.Close(jobs)
	caller.Wait(wg)

	s.Step("c2 and c3 both lead to a contradiction in 3 steps. Each worker writes its slot (worker 2 lowers best to 3, for worker 1 it is already 3), and its range ends on the closed, empty jobs: it returns.")
	w2.Write(hyps, "hyps[i]", "3 steps", slots("–", "5 steps", "3 steps", "–")...)
	shortest.Set("3")
	w2.Range(jobs)
	w2.Exit("returned")
	w1.Write(hyps, "hyps[i]", "3 steps", slots("–", "5 steps", "3 steps", "3 steps")...)
	w1.Range(jobs)
	w1.Exit("returned")

	s.Step("wg.Wait returns: every write to hyps happened before it, so the caller can read them all without a lock.")

	s.Step("The chain is picked in index order with a strict comparison: c2 beats c1, and the tie with c3 keeps the earlier c2. Which worker finished first never matters, and no chain as short as the shortest is ever given up: stopping trials changes how much is computed, not the choice.")
	pick.Set("c2, 3 steps")
	caller.Exit("returns the refutation of c2")

	return s.end()
}
