package solver

import "starbattle/internal/board"

// RunSync is the simple reference runner. Observers are treated as a priority
// list: it calls them in order, and as soon as one changes the board it starts
// again from the first. Simple rules (listed first) therefore always get the
// first chance, and expensive or harder-to-explain rules only fire when nothing
// simpler applies. It stops when a full pass changes nothing, the puzzle is
// solved, or a contradiction shows up. start is not modified.
func RunSync(start *board.State, observers []Observer) Result {
	s := start.Clone()
	active := append([]Observer(nil), observers...)
	var trace []Step

	for progress := true; progress; {
		progress = false
		next := active[:0]
		for i, o := range active {
			if o.Done(s) {
				continue // unsubscribe
			}
			next = append(next, o)

			changed := false
			for _, m := range o.Deduce(s) {
				if m.Source == "" {
					m.Source = o.Name()
				}
				delta, err := Apply(s, m)
				trace = record(trace, m, delta)
				changed = changed || len(delta) > 0
				if err != nil {
					return finish(s, trace, err)
				}
			}
			if changed {
				if err := s.Contradiction(); err != nil {
					return finish(s, trace, err)
				}
				if s.Solved() {
					return finish(s, trace, nil)
				}
				// Keep the observers not yet visited in this pass, then restart.
				next = append(next, active[i+1:]...)
				progress = true
				break
			}
		}
		active = next
	}
	return finish(s, trace, nil)
}
