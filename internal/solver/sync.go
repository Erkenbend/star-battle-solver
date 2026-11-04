package solver

import "starbattle/internal/board"

// RunSync is the simple reference runner: it calls every active observer in turn,
// applies its marks, and repeats until a full pass changes nothing, the puzzle is
// solved, or a contradiction shows up. start is not modified.
func RunSync(start *board.State, observers []Observer) Result {
	s := start.Clone()
	active := append([]Observer(nil), observers...)
	var trace []Step

	for progress := true; progress; {
		progress = false
		next := active[:0]
		for _, o := range active {
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
				progress = true
				if err := s.Contradiction(); err != nil {
					return finish(s, trace, err)
				}
				if s.Solved() {
					return finish(s, trace, nil)
				}
			}
		}
		active = next
	}
	return finish(s, trace, nil)
}
