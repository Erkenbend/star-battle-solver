package concviz

// eventsStory: section 9, party.Manager.Subscribe and the /party/{code}/events
// handler. One 1-slot channel per stream and a non-blocking send coalesce
// signals; the stream reads the current state when it wakes; each stream
// selects on its request context, the signal and a heartbeat ticker.
func eventsStory() *Story {
	s := newStory("09-party-events.svg", "9. Live party updates: coalescing signals feeding server-sent events",
		"internal/party/party.go, internal/web/party_server.go", 1280, 760)

	stream := []string{"web.partyRoutes", "party.Manager.Subscribe"}
	pub := s.Main("request: party changes", Pt{140, 200}, "party.Manager.broadcast", "party.signal")
	tick := s.Go("Tick", Pt{140, 470}, "party.Manager.Tick", "party.signal")
	hA := s.Go("stream A", Pt{820, 200}, stream...)
	hB := s.Go("stream B", Pt{820, 470}, stream...)
	ticker := s.Outside("time.Ticker", Pt{820, 335})
	browser := s.Outside("browser B", Pt{820, 610})

	changed := func(name string, at Pt) *C {
		return s.Chan(name, "struct{}", 1, at, +1, "ch", "changed").Made("party.Manager.Subscribe", "1")
	}
	chA := changed("changed (A)", Pt{480, 200})
	chB := changed("changed (B)", Pt{480, 470})
	heartbeat := func(at Pt) *C {
		return s.Chan("heartbeat.C", "time.Time", 1, at, -1, "heartbeat.C").Outside("time.NewTicker(20 s)")
	}
	hbA := heartbeat(Pt{1160, 120})
	hbB := heartbeat(Pt{1160, 390})
	ctxA := s.Chan("r.Context().Done()", "struct{}", 0, Pt{1160, 280}, -1, "r.Context().Done()").Labels('b').Outside("request of stream A")
	ctxB := s.Chan("r.Context().Done()", "struct{}", 0, Pt{1160, 550}, -1, "r.Context().Done()").Labels('b').Outside("request of stream B")

	s.Flow(pub, chA)
	s.Flow(pub, chB)
	s.Flow(tick, chA)
	s.Flow(tick, chB)
	s.Flow(chA, hA)
	s.Flow(chB, hB)
	s.Flow(hbA, hA)
	s.Flow(ctxA, hA)
	s.Flow(hbB, hB)
	s.Flow(ctxB, hB)
	s.Flow(ticker, hbA)
	s.Flow(ticker, hbB)
	s.Flow(browser, ctxB)

	wait := func(g *G, ctx, ch, hb *C) { g.Select(RecvCase(ctx), RecvCase(ch), RecvCase(hb)) }

	s.Step("Two players have the party open; each browser holds an event stream. Subscribe makes a channel with a buffer of 1 per stream, and the handler sends the current state.")
	hA.Arrive("net/http: GET …/events")
	hA.Calls("pm.Subscribe", "send", "pm.Snapshot")
	hA.Busy("sends the state, version 1")
	hB.Arrive("net/http: GET …/events")
	hB.Busy("sends the state, version 1")

	s.Step("Both wait in a select on three channels: their request's context, the change signal, and a heartbeat ticker.")
	wait(hA, ctxA, chA, hbA)
	wait(hB, ctxB, chB, hbB)

	s.Step("A player gets ready. The request, holding pm.mu, broadcasts: signal does a non-blocking send to each stream. Both wake up and send version 2.")
	pub.TrySend(chA, "{}")
	pub.TrySend(chB, "{}")
	hA.Busy("sends version 2")
	hB.Busy("sends version 2: slow client")

	s.Step("Stream A is quickly back in its select. Stream B's client is slow: B is still writing.")
	wait(hA, ctxA, chA, hbA)

	s.Step("Two changes in a burst (versions 3 and 4). A takes the first signal at once, the second waits in its slot. B's slot holds one already, so the second is dropped. The publisher never blocks.")
	pub.TrySend(chA, "{}")
	hA.Busy("sends version 3")
	pub.TrySend(chB, "{}")
	pub.TrySend(chA, "{}")
	pub.TrySend(chB, "{}")
	pub.Exit("returned")

	s.Step("Nothing is lost: a signal carries no data, and the stream reads the current state when it wakes. B finds its pending signal and sends version 4, which has both changes. So does A.")
	wait(hB, ctxB, chB, hbB)
	hB.Busy("sends version 4")
	wait(hA, ctxA, chA, hbA)
	hA.Busy("sends version 4")
	wait(hB, ctxB, chB, hbB)
	wait(hA, ctxA, chA, hbA)

	s.Step("Twenty quiet seconds later the ticker fires. A writes a comment line, a keep-alive that stops proxies from closing an idle connection.")
	ticker.Send(hbA, "tick")
	hA.Busy("writes \": ping\"")
	wait(hA, ctxA, chA, hbA)

	s.Step("Player B closes the tab: their request's context is cancelled. B's select returns, and the deferred cancel unsubscribes its channel.")
	browser.Close(ctxB)
	hB.Calls("cancel")
	hB.Exit("returned, unsubscribed")

	s.Step("Player A leaves too, the stream stays open. When the party expires, Tick signals its streams once more; A's Snapshot fails, so it sends a final gone event and returns.")
	tick.Arrive("Manager.Run: ticker")
	tick.TrySend(chA, "{}")
	tick.Exit("returned")
	hA.Busy("Snapshot fails: event gone")
	hA.Exit("returned")

	return s.end()
}
