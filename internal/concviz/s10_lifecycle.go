package concviz

// lifecycleStory: section 10, cmd/starbattle serve. A signal context, the
// party manager's ticker loop with a stop channel, the server in its own
// goroutine reporting on a 1-slot error channel, and a graceful shutdown
// (serveUntilDone, shared with design).
func lifecycleStory() *Story {
	s := newStory("10-process-lifecycle.svg", "10. Process lifecycle: signal context, ticker loop, graceful shutdown",
		"cmd/starbattle/main.go", 1280, 620)

	serve := s.Main("serve", Pt{140, 250}, "main.cmdServe", "main.serveUntilDone")
	run := s.Go("parties.Run", Pt{860, 110}, "party.Manager.Run")
	srv := s.Go("ListenAndServe", Pt{860, 280}, "main.serveUntilDone")
	sig := s.Outside("Ctrl-C or SIGTERM", Pt{140, 450})
	ticker := s.Outside("time.Ticker", Pt{1150, 250})

	errc := s.Chan("errc", "error", 1, Pt{480, 250}, -1, "errc").Made("main.serveUntilDone", "1")
	done := s.Chan("ctx.Done()", "struct{}", 0, Pt{480, 450}, +1, "ctx.Done()", "stop").Labels('b').Outside("signal.NotifyContext")
	tick := s.Chan("t.C", "time.Time", 1, Pt{1150, 110}, -1, "t.C").Outside("time.NewTicker(5 s)")

	s.Flow(srv, errc)
	s.Flow(errc, serve)
	s.Flow(sig, done)
	s.Flow(done, serve)
	s.Flow(done, run)
	s.Flow(ticker, tick)
	s.Flow(tick, run)

	s.Step("serve: signal.NotifyContext turns Ctrl-C and SIGTERM into a cancelled ctx. The party manager's loop runs as Run(5 s, ctx.Done()): a select on a ticker and a stop channel.")
	serve.Calls("signal.NotifyContext")
	serve.Spawn(run)
	run.Select(RecvCase(tick), RecvCase(done))

	s.Step("ListenAndServe blocks for the server's whole life, so it runs in a goroutine of its own and will send its error on errc, a channel with capacity 1.")
	serve.Spawn(srv)
	srv.Calls("srv.ListenAndServe")
	srv.Busy("serving requests")

	s.Step("The main goroutine waits for whichever comes first: an error of the server, or the end of ctx.")
	serve.Select(RecvCase(errc), RecvCase(done))

	s.Step("Every 5 seconds the ticker fires: Run wakes up and calls Tick (sections 7 and 9).")
	ticker.Send(tick, "tick")
	run.Calls("pm.Tick")
	run.Busy("pm.Tick()")

	s.Step("Then it selects again, until the next tick or the stop.")
	run.Select(RecvCase(tick), RecvCase(done))

	s.Step("Ctrl-C: ctx is cancelled, its Done channel closed. Both selects wake up: Run returns, serve goes on to shut down. Request contexts derive from ctx (BaseContext), so open event streams end as well.")
	sig.Close(done)
	run.Exit("returned")

	s.Step("serve calls srv.Shutdown. ListenAndServe returns ErrServerClosed and sends it on errc. Nobody reads errc any more, but its one slot takes the value, so the goroutine finishes instead of leaking.")
	serve.Calls("srv.Shutdown")
	serve.Busy("Shutdown: waits for requests")
	srv.Send(errc, "ErrClosed")
	srv.Exit("returned")

	s.Step("Shutdown returns once the open requests are done. serve returns and the process exits.")
	serve.Exit("returned")

	return s.end()
}
