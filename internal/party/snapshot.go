package party

import (
	"cmp"
	"slices"
)

// The state of a party as sent to the players, with the ranking of a round.

// Snapshot is the state of a party as sent to the players.
type Snapshot struct {
	Code      string       `json:"code"`
	Mode      string       `json:"mode"`
	Phase     string       `json:"phase"`
	Sizes     []int        `json:"sizes"`
	Lives     int          `json:"lives,omitempty"` // versus: per puzzle, 0 unlimited
	Round     int          `json:"round"`           // 0 in the lobby
	N         int          `json:"n,omitempty"`
	StartInMs int64        `json:"startInMs"` // > 0 during the countdown
	ElapsedMs int64        `json:"elapsedMs"` // since the start, while playing
	Preparing bool         `json:"preparing"` // the next puzzle is not ready yet
	PrepErr   string       `json:"prepErr,omitempty"`
	You       string       `json:"you,omitempty"` // the viewer's player ID
	Players   []SnapPlayer `json:"players"`
	Ranking   []RankEntry  `json:"ranking"`         // of the current or last round
	Coop      *CoopSnap    `json:"coop,omitempty"`  // the shared board in coop mode
	Clock     *ClockSnap   `json:"clock,omitempty"` // rules and puzzles in clock mode
	Version   int          `json:"version"`
}

// SnapPlayer is a member as seen by the others.
type SnapPlayer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Present bool   `json:"present"`
	Playing bool   `json:"playing"` // takes part in the current round
}

// RankEntry is one participant of a round.
type RankEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Rank   int    `json:"rank"`   // 0 while unranked
	Status string `json:"status"` // solved, gaveup, out (lost all lives), playing, absent, left; clock: done
	Hints  int    `json:"hints"`
	Errors int    `json:"errors,omitempty"` // versus with lives: rules broken
	TimeMs int64  `json:"timeMs"`
	Stars  int    `json:"stars"`            // clock: right minus wrong stars on the current puzzle
	Moves  int    `json:"moves,omitempty"`  // coop: changes to the shared board
	Solved int    `json:"solved,omitempty"` // clock: puzzles solved
	Skips  int    `json:"skips,omitempty"`  // clock: puzzles skipped
	Puzzle int    `json:"puzzle,omitempty"` // clock: the current puzzle, from 1
}

// Snapshot returns the state of the party as seen by the player with this
// token (or an onlooker).
func (pm *Manager) Snapshot(code, token string) (Snapshot, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, err := pm.party(code)
	if err != nil {
		return Snapshot{}, err
	}
	now := pm.now()
	s := Snapshot{
		Code:      p.Code,
		Mode:      p.Mode,
		Phase:     p.Phase,
		Sizes:     p.Sizes,
		Lives:     p.Lives,
		Preparing: len(p.ready) == 0,
		PrepErr:   p.prepErr,
		Players:   []SnapPlayer{},
		Ranking:   []RankEntry{},
		Version:   p.version,
	}
	if pl := p.byToken[token]; token != "" && pl != nil {
		s.You = pl.ID
	}
	if p.Mode == ModeClock {
		s.Clock = p.clockSnap()
	}
	r := p.Round
	for _, pl := range p.players {
		playing := r != nil && p.Phase == PhasePlaying && slices.Contains(r.Participants, pl.ID)
		s.Players = append(s.Players, SnapPlayer{pl.ID, pl.Name, pl.Ready, pl.present, playing})
	}
	if r == nil {
		return s, nil
	}
	s.Round, s.N = r.Num, r.Puzzle.N
	if p.Phase == PhasePlaying {
		if d := r.Start.Sub(now); d > 0 {
			s.StartInMs = d.Milliseconds()
		} else {
			s.ElapsedMs = (-d).Milliseconds()
		}
	}
	if r.clock != nil {
		s.Ranking = p.clockRanking()
		return s, nil
	}
	s.Ranking = p.ranking()
	if r.coop != nil {
		s.Coop = r.coopSnap(s.You)
	}
	return s, nil
}

// ranking orders the participants of the current round: solved by errors,
// then hints, then time; then those who gave up; then those still playing by progress;
// then absent and departed ones.
func (p *Party) ranking() []RankEntry {
	r := p.Round
	byID := map[string]*Player{}
	for _, pl := range p.players {
		byID[pl.ID] = pl
	}
	var out []RankEntry
	for _, id := range r.Participants {
		e := RankEntry{ID: id, Name: r.names[id]}
		pl := byID[id]
		if pl != nil {
			e.Name, e.Stars, e.Hints, e.Errors = pl.Name, pl.Stars, pl.Hints, pl.Errors
		}
		if r.coop != nil {
			e.Stars, e.Hints, e.Moves = 0, 0, r.coop.moves[id]
		}
		switch res := r.Results[id]; {
		case res != nil && res.Solved:
			e.Status, e.Hints, e.Errors, e.TimeMs, e.Stars = "solved", res.Hints, res.Errors, res.TimeMs, r.Puzzle.N
		case res != nil:
			e.Status, e.Hints, e.Errors, e.TimeMs = "gaveup", res.Hints, res.Errors, res.TimeMs
			if res.Out {
				e.Status = "out"
			}
		case pl == nil:
			e.Status = "left"
		case !pl.present:
			e.Status = "absent"
		default:
			e.Status = "playing"
		}
		out = append(out, e)
	}
	order := map[string]int{"solved": 0, "gaveup": 1, "out": 1, "playing": 2, "absent": 3, "left": 4}
	slices.SortStableFunc(out, func(a, b RankEntry) int {
		if d := cmp.Compare(order[a.Status], order[b.Status]); d != 0 {
			return d
		}
		switch a.Status {
		case "solved":
			return cmp.Or(cmp.Compare(a.Errors, b.Errors), cmp.Compare(a.Hints, b.Hints), cmp.Compare(a.TimeMs, b.TimeMs))
		case "playing":
			return cmp.Compare(b.Stars, a.Stars)
		}
		return 0
	})
	if r.coop != nil {
		return out // one result for all, no ranks
	}
	tie := func(a, b RankEntry) bool { return a.Errors == b.Errors && a.Hints == b.Hints && a.TimeMs == b.TimeMs }
	for i := range out {
		if s := out[i].Status; s == "solved" || s == "gaveup" || s == "out" {
			out[i].Rank = i + 1
			if i > 0 && s == "solved" && out[i-1].Status == "solved" && tie(out[i-1], out[i]) {
				out[i].Rank = out[i-1].Rank
			}
		}
	}
	return out
}
