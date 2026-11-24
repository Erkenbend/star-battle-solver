package concviz

// poolStory: section 6, web.Pool. A buffered channel as a work queue with a
// single consumer; a size is queued only when its puzzle is handed out, so
// the queue holds each size at most once and a send on it never blocks, not
// even under the mutex.
func poolStory() *Story {
	s := newStory("06-pool-queue.svg", "6. Pre-queueing puzzles: a buffered channel as a deduplicating queue",
		"internal/web/pool.go", 1280, 660)

	newPool := s.Main("NewPool", Pt{140, 170}, "web.NewPool")
	fill := s.Go("fill", Pt{860, 300}, "web.Pool.fill", "web.Pool.make").Background()
	req := s.Go("request: new 5×5", Pt{140, 430}, "web.Pool.Take", "web.Pool.takePuzzle")

	queue := s.Chan("queue", "int", 3, Pt{480, 300}, +1, "pl.queue").Made("web.NewPool", "len(sizes)")
	mu := s.Mutex(Pt{860, 490}, "pl.mu")
	ready := s.Box("ready: one puzzle per size", Pt{1150, 300}, 200, 80, "5: –", "6: –", "7: –")
	handed := s.Box("handed out", Pt{480, 490}, 200, 50, "–")

	s.Flow(newPool, queue)
	s.Flow(req, queue)
	s.Flow(queue, fill)
	s.Flow(fill, ready)
	s.Flow(req, handed)

	sizes := func(a, b, c string) []string { return []string{"5: " + a, "6: " + b, "7: " + c} }

	s.Step("serve calls NewPool for every size (three here: 5, 6 and 7). It queues them smallest first: the capacity is the number of sizes, so these sends never block.")
	newPool.Send(queue, "5")
	newPool.Send(queue, "6")
	newPool.Send(queue, "7")

	s.Step("It starts one fill goroutine for all sizes and returns; serve goes on. fill ranges over the queue and takes 5.")
	newPool.Spawn(fill)
	newPool.Exit("returned: serve goes on")
	fill.Range(queue)
	fill.Calls("pl.make", "generator.ForSeed", "pl.solve")
	fill.Busy("generate and solve 5×5")

	s.Step("The 5×5 puzzle is ready. fill stores it under the mutex, then takes size 6. The generator inside is already parallel, so one fill goroutine is enough.")
	fill.Lock(mu)
	fill.Write(ready, "pl.ready[n]", "5×5", sizes("ready", "–", "–")...)
	fill.Unlock(mu)
	fill.Range(queue)
	fill.Busy("generate and solve 6×6")

	s.Step("A player asks for a new 5×5 puzzle. takePuzzle takes the mutex. Meanwhile fill finishes 6×6 and has to wait for the mutex.")
	req.Arrive("net/http: new request")
	req.Lock(mu)
	fill.Lock(mu)

	s.Step("The request hands out the ready 5×5 at once and queues 5 for a refill, still holding the mutex. That send cannot block: 5 was not in the queue while its puzzle was ready.")
	ready.Set(sizes("–", "–", "–")...)
	req.Write(handed, "pl.handed[n]", "5×5", "5×5")
	req.Send(queue, "5")

	s.Step("It unlocks, which lets fill in to store 6×6. The player already has their puzzle.")
	req.Unlock(mu)
	req.Exit("puzzle sent")
	fill.Write(ready, "pl.ready[n]", "6×6", sizes("–", "ready", "–")...)
	fill.Unlock(mu)

	s.Step("fill works through the queue one size at a time: 7, then 5 again. Each size is queued at most once, so the queue never needs more room than one slot per size.")
	fill.Range(queue)
	fill.Lock(mu)
	fill.Write(ready, "pl.ready[n]", "7×7", sizes("–", "ready", "ready")...)
	fill.Unlock(mu)
	fill.Range(queue)
	fill.Lock(mu)
	fill.Write(ready, "pl.ready[n]", "5×5", sizes("ready", "ready", "ready")...)
	fill.Unlock(mu)

	s.Step("Every size is ready. The queue is empty and fill blocks in its range until the next puzzle is handed out, for as long as the server runs.")
	fill.Range(queue)

	return s.end()
}
