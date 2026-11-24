package concviz

// prepStory: section 7, party.Manager.startPrep. No channel: the preparer
// takes the manager's mutex to read and write the party, releases it around
// the slow prepare, and a generation number makes a result that outlived its
// party harmless.
func prepStory() *Story {
	s := newStory("07-party-prep.svg", "7. Preparing the next party puzzles: a mutex and a generation number, no channel",
		"internal/party/prep.go", 1280, 600)

	create := s.Go("request: create party", Pt{140, 140}, "party.Manager.Create", "party.Manager.startPrep")
	tick := s.Go("Tick", Pt{140, 420}, "party.Manager.Tick")
	prep := s.Go("preparer (gen 1)", Pt{860, 280}, "party.Manager.startPrep")

	mu := s.Mutex(Pt{500, 280}, "pm.mu")
	party := s.Box("party (behind pm.mu)", Pt{1150, 280}, 200, 80, "–")

	s.Flow(prep, party)

	state := func(gen, ready string) []string {
		return []string{"prepGen " + gen, "ready: " + ready}
	}

	s.Step("A player creates a party against the clock, which keeps 3 puzzles ready. Create holds pm.mu; startPrep bumps prepGen to 1 and starts a preparer that keeps its own copy, gen = 1.")
	create.Arrive("net/http: new request")
	create.Lock(mu)
	party.Set(state("1", "–")...)
	create.Spawn(prep)

	s.Step("The preparer needs the lock to read the party. Create still holds it, so Lock blocks until Create returns.")
	prep.Lock(mu)
	create.Unlock(mu)
	create.Exit("returned: code ABCD")

	s.Step("prepGen is still 1, so the preparation is current. It picks a size and unlocks: the slow part runs without the lock, and requests are not held up by the generator.")
	prep.Unlock(mu)
	prep.Calls("pm.prepare")
	prep.Busy("prepare(7): generate, solve")

	s.Step("The puzzle is ready. With the lock held again it checks prepGen (still 1), appends the puzzle, signals the streams (update), and goes for the next one.")
	prep.Lock(mu)
	prep.Calls("pm.update")
	party.Set(state("1", "1 puzzle")...)
	prep.Unlock(mu)
	prep.Lock(mu)
	prep.Unlock(mu)
	prep.Busy("prepare(5): generate, solve")

	s.Step("Everybody has left. Later Tick, under the lock, finds the party idle: it deletes it, bumps prepGen to 2 and signals the streams a last time.")
	tick.Arrive("Manager.Run: ticker")
	tick.Lock(mu)
	tick.Calls("signal")
	party.Set("prepGen 2", "ready: 1 puzzle", "deleted from pm.parties")
	tick.Unlock(mu)
	tick.Exit("returned")

	s.Step("prepare cannot be interrupted, but when it is done and the preparer holds the lock, prepGen 2 ≠ its gen 1: it unlocks and returns, and its puzzle is never used.")
	prep.Lock(mu)
	prep.Unlock(mu)
	prep.Drop("gen 1 ≠ 2: result dropped")
	prep.Exit("returned")

	s.Step("No channel was needed: the party lives behind pm.mu anyway. When the state is behind a mutex, a generation number is simpler than a cancel channel.")

	return s.end()
}
