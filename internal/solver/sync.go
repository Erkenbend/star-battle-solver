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

			var step Step
			step.Source = o.Name()
			for _, m := range o.Deduce(s) {
				if m.Source == "" {
					m.Source = o.Name()
				}
				delta, err := Apply(s, m)
				step.Marks = append(step.Marks, delta...)
				if err != nil {
					return finish(s, append(trace, step), err)
				}
			}
			if len(step.Marks) > 0 {
				progress = true
				trace = append(trace, step)
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
