// Package party is the party mode ("Zusammen spielen"): several players
// solve the same puzzles in rounds. A round starts when every present player is ready. The next puzzle
// is prepared in the background while a round runs.
//
// The party's creator picks one of three modes for all its rounds:
//   - versus (default): everybody solves on their own board. The round ends
//     when every present participant has solved or given up and ranks by
//     fewest errors, then fewest hints, then shortest time (server wall
//     clock from the common start). Errors count only with lives: each star
//     set where the solution has none (counted by the player's page) costs
//     one, and whoever loses all gives up.
//   - coop: everybody marks the same board, kept on the server (coop.go),
//     without hints. The round ends when the board is solved or all present
//     participants vote to give up.
//   - clock: against the clock (clock.go). Everybody solves as many puzzles
//     as possible in a set time, all the same ones in the same order, without
//     hints and with a set number of skips.
//
// Everything lives in memory behind one mutex; parties expire when idle.
//
// The package is the game only: it knows neither HTTP nor pages. Its user
// (internal/web) supplies the puzzles (Config.Prepare), carries actions and
// snapshots over the network, and builds the page of a round from Page.
// Errors are of type *Error, whose Kind the user maps to a reply.
package party

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Party limits and timing.
const (
	MaxParties = 50
	MaxPlayers = 16
	MaxName    = 20

	// Countdown is the time between "all ready" and the start of a round.
	Countdown = 3 * time.Second

	absentAfter = 60 * time.Second // without an event stream: absent
	idleExpire  = 2 * time.Hour    // without any activity: removed
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

// CodeWords are the words a party code can be besides random letters (see
// Config.Words). Each is four capital letters A to Z; I and O are fine here,
// a word is read as a word.
var CodeWords = []string{
	"NINA", "BABY", "ADRI", "MEOW", "NDIN", "YUMI", "MAMA", "PAPA", "KAKI",
	"PUPS", "NULL", "EINS", "ZWEI", "DREI", "VIER", "ACHT", "NEUN", "ZEHN",
	"PRTY", "RIPI", "STAR",
}

// Kind classifies an Error, so the caller can choose its reply.
type Kind int

// Kinds of errors.
const (
	NotFound  Kind = iota + 1 // no such party, round or puzzle
	Invalid                   // malformed request or wrong solution
	Forbidden                 // not (or no longer) a player of the party
	Conflict                  // not possible in the party's current state
	Busy                      // too many parties
)

// Error is an error of the manager. Msg is German, for the players.
type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func errorf(kind Kind, format string, args ...any) error {
	return &Error{kind, fmt.Sprintf(format, args...)}
}

var errNoParty = &Error{NotFound, "Diese Party gibt es nicht (mehr)."}

// Puzzle is a puzzle prepared for a round.
type Puzzle struct {
	N        int
	Seed     uint64
	P        *puzzle.Puzzle
	Solution []int // unique solution, star column per row
	Res      solver.Result
}

// Manager manages all parties.
type Manager struct {
	mu      sync.Mutex
	parties map[string]*Party

	minN, maxN int
	prepare    func(n int) (*Puzzle, error)
	now        func() time.Time
	pick       func(k int) int // random index below k
	retryDelay time.Duration
	words      []string                        // Config.Words, shuffled
	nextWord   int                             // index into words of the next word code
	wordTurn   bool                            // the last code was (meant to be) a word
	after      func(d time.Duration, f func()) // runs f once d has passed
}

// Config is what a Manager needs from its user. The fields after Prepare are
// for tests; their zero values mean the real thing.
type Config struct {
	// MinN and MaxN bound the sizes a party may choose.
	MinN, MaxN int
	// Prepare returns a generated and solved n×n puzzle with a unique
	// solution. It runs in the background while a round is played.
	Prepare func(n int) (*Puzzle, error)
	// Words are codes to hand out besides random ones: every second new
	// party gets the next of them, in an order New shuffles once, and
	// starting over after the last; a random code when a party has that
	// word already. Nil: only random codes. serve passes CodeWords.
	Words []string

	Now        func() time.Time                // the clock
	Pick       func(k int) int                 // a random index below k
	RetryDelay time.Duration                   // after a failed Prepare (default 5 s)
	After      func(d time.Duration, f func()) // runs f once d has passed
}

// Options are the choices of a party's creator.
type Options struct {
	Sizes []int
	Mode  string     // ModeVersus if unknown
	Clock ClockRules // for ModeClock
	// Lives per puzzle for ModeVersus: 0 is unlimited, otherwise a player's
	// Lives-th error (a wrong star) ends the round for them (so Lives-1 errors are
	// allowed). At most MaxLives.
	Lives int
}

// MaxLives bounds Options.Lives.
const MaxLives = 10

// Party is one group playing together. All fields are guarded by Manager.mu.
type Party struct {
	Code     string
	Sizes    []int
	Mode     string
	Clock    ClockRules // ModeClock only
	Lives    int        // ModeVersus only, 0: unlimited (see Options)
	Phase    string
	Round    *Round // current or last round, nil in the lobby
	players  []*Player
	byToken  map[string]*Player
	nextID   int
	ready    []*Puzzle // prepared, in the order they are played
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
	Errors   int  // wrong stars in the current round, never decreases
}

// Round is one puzzle played by the party.
type Round struct {
	Num          int
	Puzzle       *Puzzle
	Start        time.Time
	Participants []string // player IDs at the start
	Results      map[string]*Result
	names        map[string]string // participant names, also after they left
	coop         *coopBoard        // the shared board in coop mode, else nil
	clock        *clockRound       // the puzzles and scores in clock mode, else nil
}

// Result of one participant.
type Result struct {
	Solved bool          `json:"solved"`
	Out    bool          `json:"out,omitempty"` // gave up by losing all lives
	Hints  int           `json:"hints"`
	Errors int           `json:"errors"`
	Time   time.Duration `json:"-"`
	TimeMs int64         `json:"timeMs"`
}

// New returns a manager. Call Tick regularly (see Run).
func New(c Config) *Manager {
	m := &Manager{
		parties:    map[string]*Party{},
		minN:       c.MinN,
		maxN:       c.MaxN,
		prepare:    c.Prepare,
		now:        c.Now,
		pick:       c.Pick,
		retryDelay: cmp.Or(c.RetryDelay, 5*time.Second),
		after:      c.After,
		words:      slices.Clone(c.Words),
	}
	if m.now == nil {
		m.now = time.Now
	}
	if m.pick == nil {
		m.pick = mrand.IntN
	}
	for i := len(m.words) - 1; i > 0; i-- {
		j := m.pick(i + 1)
		m.words[i], m.words[j] = m.words[j], m.words[i]
	}
	if m.after == nil {
		m.after = func(d time.Duration, f func()) { time.AfterFunc(d, f) }
	}
	return m
}

// Run calls Tick every interval until stop is closed.
func (pm *Manager) Run(interval time.Duration, stop <-chan struct{}) {
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
func (pm *Manager) Create(o Options) (string, error) {
	mode := o.Mode
	if mode != ModeCoop && mode != ModeClock {
		mode = ModeVersus
	}
	var ok []int
	for _, n := range o.Sizes {
		if n >= pm.minN && n <= pm.maxN && !slices.Contains(ok, n) {
			ok = append(ok, n)
		}
	}
	if len(ok) == 0 {
		return "", errorf(Invalid, "Bitte mindestens eine Größe zwischen %d und %d wählen.", pm.minN, pm.maxN)
	}
	slices.Sort(ok)
	lives := min(max(o.Lives, 0), MaxLives)
	if mode != ModeVersus {
		lives = 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if len(pm.parties) >= MaxParties {
		return "", errorf(Busy, "Gerade laufen zu viele Partys, bitte später noch einmal.")
	}
	code := pm.newCode()
	p := &Party{
		Code:    code,
		Sizes:   ok,
		Mode:    mode,
		Clock:   o.Clock.normalize(mode),
		Lives:   lives,
		Phase:   PhaseLobby,
		byToken: map[string]*Player{},
		subs:    map[chan struct{}]bool{},
		active:  pm.now(),
	}
	pm.parties[code] = p
	pm.startPrep(p)
	return code, nil
}

// newCode returns a code that no party has: in turn the next word of
// pm.words (see Config.Words) and a random one. pm.mu must be held.
func (pm *Manager) newCode() string {
	if len(pm.words) > 0 {
		pm.wordTurn = !pm.wordTurn
		if pm.wordTurn {
			w := pm.words[pm.nextWord]
			pm.nextWord = (pm.nextWord + 1) % len(pm.words)
			if pm.parties[w] == nil {
				return w
			}
		}
	}
	for {
		var b [4]byte
		for i := range b {
			b[i] = codeLetters[pm.pick(len(codeLetters))]
		}
		if code := string(b[:]); pm.parties[code] == nil {
			return code
		}
	}
}

// NormalizeCode upper-cases a code as typed by a player.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// party returns the party with this code; pm.mu must be held.
func (pm *Manager) party(code string) (*Party, error) {
	p := pm.parties[NormalizeCode(code)]
	if p == nil {
		return nil, errNoParty
	}
	return p, nil
}

// player returns the party and the player with this token; pm.mu must be
// held. It counts as activity of the player.
func (pm *Manager) player(code, token string) (*Party, *Player, error) {
	p, err := pm.party(code)
	if err != nil {
		return nil, nil, err
	}
	pl := p.byToken[token]
	if pl == nil {
		return p, nil, errorf(Forbidden, "Du bist (nicht mehr) in dieser Party, bitte neu beitreten.")
	}
	now := pm.now()
	pl.lastSeen = now
	p.active = now
	return p, pl, nil
}

// Join adds a player, or with a known token takes the player back in (and
// renames them if name is given). It returns the token and the public ID.
func (pm *Manager) Join(code, name, token string) (string, string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > MaxName {
		name = string([]rune(name)[:MaxName])
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
		return "", "", errorf(Invalid, "Bitte einen Namen angeben.")
	}
	if len(p.players) >= MaxPlayers {
		return "", "", errorf(Conflict, "Die Party ist voll (%d Personen).", MaxPlayers)
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
func (pm *Manager) Leave(code, token string) error {
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
func (pm *Manager) SetReady(code, token string, ready bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	if p.Phase == PhasePlaying {
		return errorf(Conflict, "Die Runde läuft noch.")
	}
	pl.Ready = ready
	pm.update(p)
	return nil
}

// participant returns the running round if pl takes part in round num.
func (p *Party) participant(pl *Player, num int) (*Round, error) {
	r := p.Round
	if r == nil || r.Num != num {
		return nil, errorf(Conflict, "Das ist nicht die aktuelle Runde.")
	}
	if !slices.Contains(r.Participants, pl.ID) {
		return nil, errorf(Conflict, "Du spielst in dieser Runde nicht mit.")
	}
	return r, nil
}

// Page is what the page of a round shows: its puzzle and where it belongs.
type Page struct {
	Code   string
	Round  int
	Mode   string
	K      int // ModeClock: which of the round's puzzles, from 1; else 0
	Lives  int // ModeVersus: lives per puzzle, 0 unlimited
	Puzzle *Puzzle
}

// Page returns the page of round num, which must have started its countdown
// already. Against the clock, it is that of the round's puzzle k (from 1; 0
// counts as 1), which somebody must have reached.
func (pm *Manager) Page(code string, num, k int) (Page, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, err := pm.party(code)
	if err != nil {
		return Page{}, err
	}
	r := p.Round
	if r == nil || r.Num != num {
		return Page{}, errorf(NotFound, "Runde %d gibt es (noch) nicht.", num)
	}
	pg := Page{Code: p.Code, Round: r.Num, Mode: p.Mode, Lives: p.Lives, Puzzle: r.Puzzle}
	if c := r.clock; c != nil {
		pg.K = max(k, 1)
		if pg.K > len(c.seq) {
			return Page{}, errorf(NotFound, "Puzzle %d von Runde %d gibt es (noch) nicht.", pg.K, r.Num)
		}
		pg.Puzzle = c.seq[pg.K-1]
	}
	return pg, nil
}

// Subscribe registers an event stream of the player with this token (or an
// onlooker for token ""). The channel signals changes; cancel unregisters it.
func (pm *Manager) Subscribe(code, token string) (<-chan struct{}, func(), error) {
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
func (pm *Manager) Tick() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	now := pm.now()
	for code, p := range pm.parties {
		live := false
		for _, pl := range p.players {
			live = live || pl.conns > 0
		}
		if !live && now.Sub(p.active) > idleExpire {
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
func (pm *Manager) update(p *Party) {
	pm.refresh(p)
	pm.broadcast(p)
}

// refresh re-evaluates presence (which changes with time alone) and the
// phase, and reports whether anything changed; pm.mu must be held.
func (pm *Manager) refresh(p *Party) bool {
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

func (pm *Manager) broadcast(p *Party) {
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

func (pm *Manager) present(pl *Player, now time.Time) bool {
	return pl.conns > 0 || now.Sub(pl.lastSeen) < absentAfter
}

// maybeStart starts the next round when every present player is ready (at
// least one) and the puzzle is prepared; pm.mu must be held.
func (pm *Manager) maybeStart(p *Party) {
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
		Start:        pm.now().Add(Countdown),
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
		pl.Stars, pl.Hints, pl.Errors = 0, 0, 0
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
func (pm *Manager) maybeEnd(p *Party) {
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

// Subscribers is the number of open event streams of the party, 0 if there
// is no such party.
func (pm *Manager) Subscribers(code string) int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p := pm.parties[NormalizeCode(code)]; p != nil {
		return len(p.subs)
	}
	return 0
}
