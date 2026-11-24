package party

import (
	"log"
	"time"
)

// Preparing puzzles: a party keeps the next ones ready, generated in the
// background while a round runs (see architecture/concurrency.md section 7).

// prepTarget is how many prepared puzzles a party keeps ready: against the
// clock a few, so that fast players rarely wait for the generator.
func (p *Party) prepTarget() int {
	if p.Mode == ModeClock {
		return clockAhead
	}
	return 1
}

// startPrep prepares puzzles in the background until the party has enough
// ready (prepTarget); pm.mu must be held.
func (pm *Manager) startPrep(p *Party) {
	if p.prepping || len(p.ready) >= p.prepTarget() {
		return
	}
	p.prepping = true
	p.prepGen++
	gen := p.prepGen
	go func() {
		for try := 0; ; try++ {
			pm.mu.Lock()
			if p.prepGen != gen {
				pm.mu.Unlock()
				return
			}
			if len(p.ready) >= p.prepTarget() {
				p.prepping = false
				pm.mu.Unlock()
				return
			}
			n := p.Sizes[pm.pick(len(p.Sizes))]
			if len(p.Sizes) > 1 && n == p.lastSize && try == 0 {
				n = p.Sizes[pm.pick(len(p.Sizes))] // a second draw makes repeats rarer
			}
			pm.mu.Unlock()

			start := pm.now()
			rp, err := pm.prepare(n)

			pm.mu.Lock()
			if p.prepGen != gen {
				pm.mu.Unlock()
				return
			}
			if err == nil {
				log.Printf("Party %s: %d×%d bereit (Nr. %d, %.1f s)", p.Code, n, n, rp.Seed, pm.now().Sub(start).Seconds())
				p.ready = append(p.ready, rp)
				p.prepErr, p.lastSize = "", n
				try = -1 // the next puzzle starts afresh
				pm.update(p)
				pm.mu.Unlock()
				continue
			}
			log.Printf("Party %s: %d×%d: %v", p.Code, n, n, err)
			p.prepErr = "Das nächste Puzzle ließ sich nicht erzeugen, neuer Versuch läuft."
			pm.update(p)
			pm.mu.Unlock()
			time.Sleep(pm.retryDelay)
		}
	}()
}
