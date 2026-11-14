package web

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	mrand "math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Party mode ("Zusammen spielen"): several players solve the same puzzles in
// rounds. A round starts when every present player is ready. The next puzzle
// is prepared in the background while a round runs.
//
// The party's creator picks one of two modes for all its rounds:
//   - versus (default): everybody solves on their own board. The round ends
//     when every present participant has solved or given up and ranks by
//     fewest hints, then shortest time (server wall clock from the common
//     start).
//   - coop: everybody marks the same board, kept on the server (coop.go),
//     without hints. The round ends when the board is solved or all present
//     participants vote to give up.
//   - clock: against the clock (clock.go). Everybody solves as many puzzles
//     as possible in a set time, all the same ones in the same order, without
//     hints and with a set number of skips.
//
// Everything lives in memory behind one mutex; parties expire when idle.

// Party limits and timing.
const (
	PartyMaxParties = 50
	PartyMaxPlayers = 16
	PartyMaxName    = 20

	partyAbsentAfter = 60 * time.Second // without an event stream: absent
	partyIdleExpire  = 2 * time.Hour    // without any activity: removed
	partyCountdown   = 3 * time.Second  // between "all ready" and the start
)

// Party modes.
const (
	ModeVersus = "versus" // each on their own board, ranked
	ModeCoop   = "coop"   // all on the same board
	ModeClock  = "clock"  // as many puzzles as possible in a set time
)

// Party phases.
const (
	PhaseLobby   = "lobby"   // before the first round
	PhasePlaying = "playing" // a round runs (or counts down)
	PhaseResults = "results" // a round has ended, waiting for "ready"
)

// codeLetters are the characters of a party code: no I and O, which look
// like 1 and 0.
const codeLetters = "ABCDEFGHJKLMNPQRSTUVWXYZ"

// PartyError is an error with the HTTP status the handler answers with.
type PartyError struct {
	Status int
	Msg    string
}

func (e *PartyError) Error() string { return e.Msg }

func partyErr(status int, format string, args ...any) error {
	return &PartyError{status, fmt.Sprintf(format, args...)}
}

var errNoParty = &PartyError{http.StatusNotFound, "Diese Party gibt es nicht (mehr)."}

// RoundPuzzle is a puzzle prepared for a round.
type RoundPuzzle struct {
	N        int
	Seed     uint64
	P        *puzzle.Puzzle
	Solution []int // unique solution, star column per row
	Res      solver.Result
}

// Parties manages all parties.
type Parties struct {
	mu      sync.Mutex
	parties map[string]*Party

	// Replaceable in tests.
	now        func() time.Time
	prepare    func(n int) (*RoundPuzzle, error)
	pick       func(k int) int // random index below k
	retryDelay time.Duration
	after      func(d time.Duration, f func()) // runs f once d has passed
}

// PartyOptions are the choices of a party's creator.
type PartyOptions struct {
	Sizes []int
	Mode  string     // ModeVersus if unknown
	Clock ClockRules // for ModeClock
}

// Party is one group playing together. All fields are guarded by Parties.mu.
type Party struct {
	Code     string
	Sizes    []int
	Mode     string
	Clock    ClockRules // ModeClock only
	Phase    string
	Round    *Round // current or last round, nil in the lobby
	players  []*Player
	byToken  map[string]*Player
	nextID   int
	ready    []*RoundPuzzle // prepared, in the order they are played
	prepping bool
	prepGen  int // discards a preparation that outlived its party
	prepErr  string
	lastSize int
	version  int
	subs     map[chan struct{}]bool
	active   time.Time // last activity, for expiry
}

// Player is a member of a party.
type Player struct {
	ID       string // public, in snapshots
	token    string // secret, identifies the player's requests
	Name     string
	Ready    bool
	conns    int // open event streams
	lastSeen time.Time
	present  bool // as of the last check, to notice changes
	Stars    int  // progress in the current round
	Hints    int  // hints in the current round, never decreases
}

// Round is one puzzle played by the party.
type Round struct {
	Num          int
	Puzzle       *RoundPuzzle
	Start        time.Time
	Participants []string // player IDs at the start
	Results      map[string]*Result
	names        map[string]string // participant names, also after they left
	data         *Data             // page data, built on first request
	coop         *coopBoard        // the shared board in coop mode, else nil
	clock        *clockRound       // the puzzles and scores in clock mode, else nil
}

// Result of one participant.
type Result struct {
	Solved bool          `json:"solved"`
	Hints  int           `json:"hints"`
	Time   time.Duration `json:"-"`
	TimeMs int64         `json:"timeMs"`
}

// NewParties returns a manager that prepares puzzles with solve, taking ready
// ones from pool (may be nil) first. Call Tick regularly (see Run).
func NewParties(solve SolveFunc, pool *Pool) *Parties {
	return &Parties{
		parties: map[string]*Party{},
		now:     time.Now,
		prepare: func(n int) (*RoundPuzzle, error) {
			if e := pool.takePuzzle(n); e != nil && e.sol != nil {
				return &RoundPuzzle{n, e.seed, e.p, e.sol, e.res}, nil
			}
			seed := newSeed()
			p, sol, err := generate(n, seed)
			if err != nil {
				return nil, err
			}
			if sol == nil {
				return nil, fmt.Errorf("%d×%d Nr. %d: keine eindeutige Lösung", n, n, seed)
			}
			res, err := solve(p)
			if err != nil {
				return nil, err
			}
			return &RoundPuzzle{n, seed, p, sol, res}, nil
		},
		pick:       func(k int) int { return mrand.IntN(k) },
		retryDelay: 5 * time.Second,
		after:      func(d time.Duration, f func()) { time.AfterFunc(d, f) },
	}
}

// Run calls Tick every interval until stop is closed.
func (pm *Parties) Run(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			pm.Tick()
		case <-stop:
			return
		}
	}
}

// Create starts a new party with the chosen options and returns its code.
func (pm *Parties) Create(o PartyOptions) (string, error) {
	mode := o.Mode
	if mode != ModeCoop && mode != ModeClock {
		mode = ModeVersus
	}
	var ok []int
	for _, n := range o.Sizes {
		if n >= MinN && n <= MaxN && !slices.Contains(ok, n) {
			ok = append(ok, n)
		}
	}
	if len(ok) == 0 {
		return "", partyErr(http.StatusBadRequest, "Bitte mindestens eine Größe zwischen %d und %d wählen.", MinN, MaxN)
	}
	slices.Sort(ok)
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if len(pm.parties) >= PartyMaxParties {
		return "", partyErr(http.StatusServiceUnavailable, "Gerade laufen zu viele Partys, bitte später noch einmal.")
	}
	code := ""
	for code == "" || pm.parties[code] != nil {
		var b [4]byte
		for i := range b {
			b[i] = codeLetters[pm.pick(len(codeLetters))]
		}
		code = string(b[:])
	}
	p := &Party{
		Code:    code,
		Sizes:   ok,
		Mode:    mode,
		Clock:   o.Clock.normalize(mode),
		Phase:   PhaseLobby,
		byToken: map[string]*Player{},
		subs:    map[chan struct{}]bool{},
		active:  pm.now(),
	}
	pm.parties[code] = p
	pm.startPrep(p)
	return code, nil
}

// NormalizeCode upper-cases a code as typed by a player.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// party returns the party with this code; pm.mu must be held.
func (pm *Parties) party(code string) (*Party, error) {
	p := pm.parties[NormalizeCode(code)]
	if p == nil {
		return nil, errNoParty
	}
	return p, nil
}

// player returns the party and the player with this token; pm.mu must be
// held. It counts as activity of the player.
func (pm *Parties) player(code, token string) (*Party, *Player, error) {
	p, err := pm.party(code)
	if err != nil {
		return nil, nil, err
	}
	pl := p.byToken[token]
	if pl == nil {
		return p, nil, partyErr(http.StatusForbidden, "Du bist (nicht mehr) in dieser Party, bitte neu beitreten.")
	}
	now := pm.now()
	pl.lastSeen = now
	p.active = now
	return p, pl, nil
}

// Join adds a player, or with a known token takes the player back in (and
// renames them if name is given). It returns the token and the public ID.
func (pm *Parties) Join(code, name, token string) (string, string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > PartyMaxName {
		name = string([]rune(name)[:PartyMaxName])
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, err := pm.party(code)
	if err != nil {
		return "", "", err
	}
	now := pm.now()
	if pl := p.byToken[token]; token != "" && pl != nil {
		if name != "" {
			pl.Name = name
		}
		pl.lastSeen = now
		p.active = now
		pm.update(p)
		return pl.token, pl.ID, nil
	}
	if name == "" {
		return "", "", partyErr(http.StatusBadRequest, "Bitte einen Namen angeben.")
	}
	if len(p.players) >= PartyMaxPlayers {
		return "", "", partyErr(http.StatusConflict, "Die Party ist voll (%d Personen).", PartyMaxPlayers)
	}
	p.nextID++
	pl := &Player{ID: fmt.Sprintf("p%d", p.nextID), token: newToken(), Name: name, lastSeen: now, present: true}
	p.players = append(p.players, pl)
	p.byToken[pl.token] = pl
	p.active = now
	// Joining during the countdown still makes it into the round.
	if r := p.Round; p.Phase == PhasePlaying && now.Before(r.Start) {
		r.Participants = append(r.Participants, pl.ID)
		r.names[pl.ID] = pl.Name
	}
	pm.update(p)
	return pl.token, pl.ID, nil
}

func newToken() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Leave removes the player from the party. Their result stays in the round.
func (pm *Parties) Leave(code, token string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	p.players = slices.DeleteFunc(p.players, func(q *Player) bool { return q == pl })
	delete(p.byToken, token)
	pm.update(p)
	return nil
}

// SetReady marks the player (not) ready for the next round.
func (pm *Parties) SetReady(code, token string, ready bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	if p.Phase == PhasePlaying {
		return partyErr(http.StatusConflict, "Die Runde läuft noch.")
	}
	pl.Ready = ready
	pm.update(p)
	return nil
}

// participant returns the running round if pl takes part in round num.
func (p *Party) participant(pl *Player, num int) (*Round, error) {
	r := p.Round
	if r == nil || r.Num != num {
		return nil, partyErr(http.StatusConflict, "Das ist nicht die aktuelle Runde.")
	}
	if !slices.Contains(r.Participants, pl.ID) {
		return nil, partyErr(http.StatusConflict, "Du spielst in dieser Runde nicht mit.")
	}
	return r, nil
}

// versusRound returns the running round num if pl takes part and it is not
// played against the clock, where each puzzle counts on its own.
func (p *Party) versusRound(pl *Player, num int) (*Round, error) {
	r, err := p.participant(pl, num)
	if err != nil {
		return nil, err
	}
	if r.clock != nil {
		return nil, partyErr(http.StatusConflict, "In dieser Party wird gegen die Uhr gespielt.")
	}
	return r, nil
}

// Progress reports the stars set and hints taken in round num.
func (pm *Parties) Progress(code, token string, num, stars, hints int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	if _, err := p.versusRound(pl, num); err != nil {
		return err
	}
	if p.Phase != PhasePlaying {
		return nil
	}
	hints = max(hints, pl.Hints)
	if stars != pl.Stars || hints != pl.Hints {
		pl.Stars, pl.Hints = stars, hints
		pm.update(p)
	}
	return nil
}

// Solve records a solution of round num (star column per row). Repeating a
// successful call returns the same result.
func (pm *Parties) Solve(code, token string, num int, stars []int, hints int) (Result, error) {
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
		return Result{}, partyErr(http.StatusConflict, "Du hast diese Runde aufgegeben.")
	}
	if p.Phase != PhasePlaying {
		return Result{}, partyErr(http.StatusConflict, "Die Runde ist schon vorbei.")
	}
	now := pm.now()
	if now.Before(r.Start) {
		return Result{}, partyErr(http.StatusConflict, "Die Runde hat noch nicht begonnen.")
	}
	if !slices.Equal(stars, r.Puzzle.Solution) {
		return Result{}, partyErr(http.StatusBadRequest, "Das ist nicht die Lösung.")
	}
	t := now.Sub(r.Start)
	res := &Result{Solved: true, Hints: max(hints, pl.Hints), Time: t, TimeMs: t.Milliseconds()}
	r.Results[pl.ID] = res
	pl.Hints, pl.Stars = res.Hints, r.Puzzle.N
	pm.update(p)
	return *res, nil
}

// GiveUp ends round num for the player without a solution.
func (pm *Parties) GiveUp(code, token string, num, hints int) error {
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
			return partyErr(http.StatusConflict, "Du hast diese Runde schon gelöst.")
		}
		return nil
	}
	if p.Phase != PhasePlaying {
		return partyErr(http.StatusConflict, "Die Runde ist schon vorbei.")
	}
	t := max(pm.now().Sub(r.Start), 0)
	r.Results[pl.ID] = &Result{Hints: max(hints, pl.Hints), Time: t, TimeMs: t.Milliseconds()}
	pm.update(p)
	return nil
}

// RoundData returns the page data of round num, which must have started its
// countdown already. Against the clock, it is that of the round's puzzle k
// (from 1; 0 counts as 1).
func (pm *Parties) RoundData(code string, num, k int) (Data, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, err := pm.party(code)
	if err != nil {
		return Data{}, err
	}
	r := p.Round
	if r == nil || r.Num != num {
		return Data{}, partyErr(http.StatusNotFound, "Runde %d gibt es (noch) nicht.", num)
	}
	if r.clock != nil {
		return p.clockData(r, max(k, 1))
	}
	if r.data == nil {
		rp := r.Puzzle
		d := Build(fmt.Sprintf("Party %s · Runde %d · %d×%d", p.Code, r.Num, rp.N, rp.N), rp.P, rp.Solution, rp.Res)
		d.Served = true
		d.Text = rp.P.String()
		r.data = &d
	}
	d := *r.data
	d.Party = &PartyInfo{Code: p.Code, Round: r.Num, Mode: p.Mode}
	return d, nil
}

// Subscribe registers an event stream of the player with this token (or an
// onlooker for token ""). The channel signals changes; cancel unregisters it.
func (pm *Parties) Subscribe(code, token string) (<-chan struct{}, func(), error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, err := pm.party(code)
	if err != nil {
		return nil, nil, err
	}
	pl := p.byToken[token]
	ch := make(chan struct{}, 1)
	p.subs[ch] = true
	if pl != nil {
		pl.conns++
		pl.lastSeen = pm.now()
		pm.update(p)
	}
	cancel := func() {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		delete(p.subs, ch)
		if pl != nil {
			pl.conns--
			pl.lastSeen = pm.now()
		}
	}
	return ch, cancel, nil
}

// Tick notices absent players, starts and ends rounds accordingly and
// removes idle parties.
func (pm *Parties) Tick() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	now := pm.now()
	for code, p := range pm.parties {
		live := false
		for _, pl := range p.players {
			live = live || pl.conns > 0
		}
		if !live && now.Sub(p.active) > partyIdleExpire {
			delete(pm.parties, code)
			p.prepGen++ // a running preparation is dropped
			for ch := range p.subs {
				signal(ch) // streams notice the party is gone
			}
			continue
		}
		if pm.refresh(p) {
			pm.broadcast(p)
		}
	}
}

// update follows every change of a party: it re-evaluates presence and the
// phase and signals the event streams; pm.mu must be held.
func (pm *Parties) update(p *Party) {
	pm.refresh(p)
	pm.broadcast(p)
}

// refresh re-evaluates presence (which changes with time alone) and the
// phase, and reports whether anything changed; pm.mu must be held.
func (pm *Parties) refresh(p *Party) bool {
	now := pm.now()
	changed := false
	for _, pl := range p.players {
		if pres := pm.present(pl, now); pres != pl.present {
			pl.present = pres
			if !pres {
				pl.Ready = false
			}
			changed = true
		}
	}
	phase, round := p.Phase, p.Round
	n := 0
	if round != nil {
		n = len(round.Participants)
	}
	seq := 0
	if round != nil && round.clock != nil {
		seq = len(round.clock.seq)
	}
	p.enroll()
	p.clockFeed()
	pm.maybeEnd(p)
	pm.maybeStart(p)
	pm.startPrep(p)
	if round != nil && round.clock != nil {
		changed = changed || len(round.clock.seq) != seq
	}
	return changed || p.Phase != phase || p.Round != round || (round != nil && len(round.Participants) != n)
}

func (pm *Parties) broadcast(p *Party) {
	p.version++
	for ch := range p.subs {
		signal(ch)
	}
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (pm *Parties) present(pl *Player, now time.Time) bool {
	return pl.conns > 0 || now.Sub(pl.lastSeen) < partyAbsentAfter
}

// maybeStart starts the next round when every present player is ready (at
// least one) and the puzzle is prepared; pm.mu must be held.
func (pm *Parties) maybeStart(p *Party) {
	if p.Phase == PhasePlaying || len(p.ready) == 0 {
		return
	}
	var ids []string
	for _, pl := range p.players {
		if !pl.present {
			continue
		}
		if !pl.Ready {
			return
		}
		ids = append(ids, pl.ID)
	}
	if len(ids) == 0 {
		return
	}
	num := 1
	if p.Round != nil {
		num = p.Round.Num + 1
	}
	next := p.ready[0]
	p.ready = slices.Delete(p.ready, 0, 1)
	r := &Round{
		Num:          num,
		Puzzle:       next,
		Start:        pm.now().Add(partyCountdown),
		Participants: ids,
		Results:      map[string]*Result{},
		names:        map[string]string{},
	}
	switch p.Mode {
	case ModeCoop:
		r.coop = newCoopBoard(next.N)
	case ModeClock:
		r.clock = newClockRound(p.Clock, r.Start, next)
		pm.after(r.clock.deadline.Sub(pm.now()), func() { // ends the round on time
			pm.mu.Lock()
			defer pm.mu.Unlock()
			if pm.parties[p.Code] == p { // not expired in the meantime
				pm.update(p)
			}
		})
	}
	for _, pl := range p.players {
		pl.Ready = false
		pl.Stars, pl.Hints = 0, 0
		if pl.present {
			r.names[pl.ID] = pl.Name
		}
	}
	p.Round = r
	p.Phase = PhasePlaying
	pm.startPrep(p)
}

// maybeEnd ends the round when every present participant has a result, and
// at least one result exists or somebody else waits; pm.mu must be held.
func (pm *Parties) maybeEnd(p *Party) {
	r := p.Round
	if p.Phase != PhasePlaying || pm.now().Before(r.Start) {
		return
	}
	if r.coop != nil {
		pm.coopMaybeEnd(p)
		return
	}
	if r.clock != nil {
		if !pm.now().Before(r.clock.deadline) {
			p.Phase = PhaseResults
		}
		return
	}
	waiting := false
	for _, pl := range p.players {
		if !pl.present {
			continue
		}
		if slices.Contains(r.Participants, pl.ID) {
			if r.Results[pl.ID] == nil {
				return
			}
		} else {
			waiting = true
		}
	}
	if len(r.Results) == 0 && !waiting {
		return // everybody is away: keep the round for when they come back
	}
	p.Phase = PhaseResults
}

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
func (pm *Parties) startPrep(p *Party) {
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

// Snapshot is the state of a party as sent to the players.
type Snapshot struct {
	Code      string       `json:"code"`
	Mode      string       `json:"mode"`
	Phase     string       `json:"phase"`
	Sizes     []int        `json:"sizes"`
	Round     int          `json:"round"` // 0 in the lobby
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
	Status string `json:"status"` // solved, gaveup, playing, absent, left; clock: done
	Hints  int    `json:"hints"`
	TimeMs int64  `json:"timeMs"`
	Stars  int    `json:"stars"`            // clock: right minus wrong stars on the current puzzle
	Moves  int    `json:"moves,omitempty"`  // coop: changes to the shared board
	Solved int    `json:"solved,omitempty"` // clock: puzzles solved
	Skips  int    `json:"skips,omitempty"`  // clock: puzzles skipped
	Puzzle int    `json:"puzzle,omitempty"` // clock: the current puzzle, from 1
}

// Snapshot returns the state of the party as seen by the player with this
// token (or an onlooker).
func (pm *Parties) Snapshot(code, token string) (Snapshot, error) {
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

// ranking orders the participants of the current round: solved by hints,
// then time; then those who gave up; then those still playing by progress;
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
			e.Name, e.Stars, e.Hints = pl.Name, pl.Stars, pl.Hints
		}
		if r.coop != nil {
			e.Stars, e.Hints, e.Moves = 0, 0, r.coop.moves[id]
		}
		switch res := r.Results[id]; {
		case res != nil && res.Solved:
			e.Status, e.Hints, e.TimeMs, e.Stars = "solved", res.Hints, res.TimeMs, r.Puzzle.N
		case res != nil:
			e.Status, e.Hints, e.TimeMs = "gaveup", res.Hints, res.TimeMs
		case pl == nil:
			e.Status = "left"
		case !pl.present:
			e.Status = "absent"
		default:
			e.Status = "playing"
		}
		out = append(out, e)
	}
	order := map[string]int{"solved": 0, "gaveup": 1, "playing": 2, "absent": 3, "left": 4}
	slices.SortStableFunc(out, func(a, b RankEntry) int {
		if d := cmp.Compare(order[a.Status], order[b.Status]); d != 0 {
			return d
		}
		switch a.Status {
		case "solved":
			return cmp.Or(cmp.Compare(a.Hints, b.Hints), cmp.Compare(a.TimeMs, b.TimeMs))
		case "playing":
			return cmp.Compare(b.Stars, a.Stars)
		}
		return 0
	})
	if r.coop != nil {
		return out // one result for all, no ranks
	}
	for i := range out {
		if s := out[i].Status; s == "solved" || s == "gaveup" {
			out[i].Rank = i + 1
			if i > 0 && out[i-1].Status == "solved" && s == "solved" && out[i-1].Hints == out[i].Hints && out[i-1].TimeMs == out[i].TimeMs {
				out[i].Rank = out[i-1].Rank
			}
		}
	}
	return out
}

// errorStatus returns the HTTP status for an error of the manager.
func errorStatus(err error) int {
	if pe, ok := errors.AsType[*PartyError](err); ok {
		return pe.Status
	}
	return http.StatusInternalServerError
}
