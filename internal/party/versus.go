package party

import "slices"

// Rounds against each other ("Gegeneinander", the default mode): everybody
// solves the round's puzzle on their own board and reports progress, a
// solution or giving up; with lives, too many errors give up.

// versusRound returns the running round num if pl takes part and it is not
// played against the clock, where each puzzle counts on its own.
func (p *Party) versusRound(pl *Player, num int) (*Round, error) {
	r, err := p.participant(pl, num)
	if err != nil {
		return nil, err
	}
	if r.clock != nil {
		return nil, errorf(Conflict, "In dieser Party wird gegen die Uhr gespielt.")
	}
	return r, nil
}

// Progress reports the stars set, hints taken and rules broken in round num.
// Breaking the rules once too often (see Options.Lives) gives up.
func (pm *Manager) Progress(code, token string, num, stars, hints, errors int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	r, err := p.versusRound(pl, num)
	if err != nil {
		return err
	}
	if p.Phase != PhasePlaying || r.Results[pl.ID] != nil {
		return nil
	}
	hints, errors = max(hints, pl.Hints), max(errors, pl.Errors)
	if stars != pl.Stars || hints != pl.Hints || errors != pl.Errors {
		pl.Stars, pl.Hints, pl.Errors = stars, hints, errors
		pm.outOfLives(p, pl, r)
		pm.update(p)
	}
	return nil
}

// outOfLives gives up for pl, if they have used up their lives and have no
// result yet, and reports whether they are out; pm.mu must be held.
func (pm *Manager) outOfLives(p *Party, pl *Player, r *Round) bool {
	if p.Lives == 0 || pl.Errors < p.Lives {
		return false
	}
	if r.Results[pl.ID] == nil {
		t := max(pm.now().Sub(r.Start), 0)
		r.Results[pl.ID] = &Result{Out: true, Hints: pl.Hints, Errors: pl.Errors, Time: t, TimeMs: t.Milliseconds()}
	}
	return true
}

// Solve records a solution of round num (star column per row). Repeating a
// successful call returns the same result. Errors are the rules broken on
// the way; one too many (see Options.Lives) gives up instead.
func (pm *Manager) Solve(code, token string, num int, stars []int, hints, errors int) (Result, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return Result{}, err
	}
	r, err := p.versusRound(pl, num)
	if err != nil {
		return Result{}, err
	}
	if res := r.Results[pl.ID]; res != nil {
		if res.Solved {
			return *res, nil
		}
		if res.Out {
			return Result{}, errorf(Conflict, "Du hast alle Leben verloren.")
		}
		return Result{}, errorf(Conflict, "Du hast diese Runde aufgegeben.")
	}
	if p.Phase != PhasePlaying {
		return Result{}, errorf(Conflict, "Die Runde ist schon vorbei.")
	}
	now := pm.now()
	if now.Before(r.Start) {
		return Result{}, errorf(Conflict, "Die Runde hat noch nicht begonnen.")
	}
	pl.Hints, pl.Errors = max(hints, pl.Hints), max(errors, pl.Errors)
	if pm.outOfLives(p, pl, r) {
		pm.update(p)
		return Result{}, errorf(Conflict, "Du hast alle Leben verloren.")
	}
	if !slices.Equal(stars, r.Puzzle.Solution) {
		return Result{}, errorf(Invalid, "Das ist nicht die Lösung.")
	}
	t := now.Sub(r.Start)
	res := &Result{Solved: true, Hints: pl.Hints, Errors: pl.Errors, Time: t, TimeMs: t.Milliseconds()}
	r.Results[pl.ID] = res
	pl.Stars = r.Puzzle.N
	pm.update(p)
	return *res, nil
}

// GiveUp ends round num for the player without a solution; with errors that
// use up the lives (see Options.Lives) as having lost them.
func (pm *Manager) GiveUp(code, token string, num, hints, errors int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	r, err := p.versusRound(pl, num)
	if err != nil {
		return err
	}
	if res := r.Results[pl.ID]; res != nil {
		if res.Solved {
			return errorf(Conflict, "Du hast diese Runde schon gelöst.")
		}
		return nil
	}
	if p.Phase != PhasePlaying {
		return errorf(Conflict, "Die Runde ist schon vorbei.")
	}
	pl.Hints, pl.Errors = max(hints, pl.Hints), max(errors, pl.Errors)
	if !pm.outOfLives(p, pl, r) {
		t := max(pm.now().Sub(r.Start), 0)
		r.Results[pl.ID] = &Result{Hints: pl.Hints, Errors: pl.Errors, Time: t, TimeMs: t.Milliseconds()}
	}
	pm.update(p)
	return nil
}
