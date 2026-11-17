package concviz

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Sizes of the scene, in SVG units.
const (
	gW, gH    = 226.0, 72.0 // goroutine box
	chH       = 34.0        // channel
	slotW     = 76.0        // one buffer slot
	chPad     = 10.0
	unbufW    = 96.0 // an unbuffered channel
	tokH      = 20.0
	maxTok    = 9  // characters of a token label
	maxStatus = 32 // characters of a status line
)

// Pt is a point of the scene.
type Pt struct{ X, Y float64 }

type rect struct{ x, y, w, h float64 }

func around(c Pt, w, h float64) rect { return rect{c.X - w/2, c.Y - h/2, w, h} }

func (r rect) center() Pt { return Pt{r.x + r.w/2, r.y + r.h/2} }

// border is where the ray from r's centre towards p leaves r.
func (r rect) border(p Pt) Pt {
	c := r.center()
	dx, dy := p.X-c.X, p.Y-c.Y
	t := math.Inf(1)
	if dx != 0 {
		t = r.w / 2 / math.Abs(dx)
	}
	if dy != 0 {
		t = min(t, r.h/2/math.Abs(dy))
	}
	if math.IsInf(t, 1) {
		return c
	}
	return Pt{c.X + dx*t, c.Y + dy*t}
}

// node is something in the scene: a goroutine, a channel or a box.
type node interface {
	bounds() rect
	label() string
}

// Story is one animation: a scene and the steps played on it.
type Story struct {
	File   string // file name in architecture/concurrency/
	Title  string
	Source string // the file the story is about, shown in the corner
	W, H   float64

	gs     []*G
	cs     []*C
	boxes  []*Box
	toks   []*Tok
	flows  [][2]node
	frames []frame // frames[0]: before the first step; frames[i]: after step i
	rows   []row   // the events, for the sequence diagram
	needs  []need
	seen   map[string]bool
	cap    string   // caption of the step being scripted
	ops    []string // what happened in it, in words
	inDo   bool     // inside the function of a first once.Do
}

func newStory(file, title, source string, w, h float64) *Story {
	return &Story{File: file, Title: title, Source: source, W: w, H: h, seen: map[string]bool{}}
}

func (s *Story) fail(format string, a ...any) {
	panic(fmt.Sprintf("%s: %s", s.File, fmt.Sprintf(format, a...)))
}

// Flow draws a fixed arrow for the way values take.
func (s *Story) Flow(from, to node) { s.flows = append(s.flows, [2]node{from, to}) }

func (s *Story) logf(format string, a ...any) { s.ops = append(s.ops, fmt.Sprintf(format, a...)) }

// Step ends the step scripted so far and starts the next one with caption.
// The first call ends the setup of the scene.
func (s *Story) Step(caption string) {
	ended := len(s.frames) // 0: the setup
	if ended > 0 {
		if k := len(s.rows); k == 0 || s.rows[k-1].step != ended {
			s.rows = append(s.rows, row{step: ended}) // a step without events still gets a row
		}
		s.rows[len(s.rows)-1].after = s.states()
	}
	f := s.capture()
	f.caption, f.ops = s.cap, s.ops
	s.frames = append(s.frames, f)
	s.cap, s.ops = caption, nil
	for _, t := range s.toks {
		t.start, t.alive0, t.born, t.via = t.pos(), !t.gone, false, nil
	}
}

// end ends the last step and checks that no goroutine is left behind.
func (s *Story) end() *Story {
	s.Step("")
	for _, g := range s.gs {
		if !g.outside && !g.background && (g.st == running || g.st == blocked) {
			s.fail("%s is still %s at the end", g.name, g.st)
		}
	}
	for _, c := range s.cs {
		if !c.made && !c.outside {
			s.fail("channel %s: neither Made nor Outside", c.name)
		}
	}
	return s
}

// --- events, for the sequence diagram ---

type evKind uint8

const (
	evSpawn   evKind = iota // from starts to (from nil: started from outside)
	evSend                  // from puts a value into the buffer of channel to
	evHandoff               // from hands a value through channel via to the waiting to
	evRecv                  // to takes a value from the buffer of channel from
	evClose                 // from closes channel to
	evClosed                // to receives from the closed channel from
	evGive                  // from puts a value into box to
	evTake                  // to takes a value from box from
	evNote                  // something happens at from, e.g. a default case
	evExit                  // from returns; to is its WaitGroup, if any
	evWake                  // the WaitGroup or mutex from lets to's Wait or Lock return
	evLock                  // from locks the mutex to
	evUnlock                // from unlocks the mutex to
)

type event struct {
	kind          evKind
	from, via, to node
	label         string
	tok           *Tok // a value entering or leaving a buffer
	wg            *WG  // a WaitGroup whose counter the event changes, to n
	n             int
}

// row is one event, or an empty row for a step without events, with the
// states of the goroutines after it.
type row struct {
	step  int
	ev    *event
	after []gFrame
}

func (s *Story) ev(e event) {
	if k := len(s.rows); k > 0 && s.rows[k-1].after == nil {
		s.rows[k-1].after = s.states()
	}
	s.rows = append(s.rows, row{step: len(s.frames), ev: &e})
}

func (s *Story) states() []gFrame {
	var out []gFrame
	for _, g := range s.gs {
		out = append(out, gFrame{g.st, g.status})
	}
	return out
}

// --- goroutines ---

type state uint8

const (
	hidden state = iota // not started yet
	running
	blocked
	exited
)

func (st state) String() string {
	return [...]string{"not started", "running", "blocked", "exited"}[st]
}

// G is a goroutine.
type G struct {
	s          *Story
	name       string
	fns        []string // functions its code is in, as "pkg.Func" or "pkg.Type.Method"
	at         Pt
	outside    bool // stands for something outside the code (a signal, a browser)
	background bool // runs as long as the process: may be left running or blocked
	st         state
	status     string
	hand       []*Tok
	w          *waiter // what it is blocked on
	wg         *WG     // started by this WaitGroup's Go
}

func (s *Story) addG(name string, at Pt, fns []string, st state, outside bool) *G {
	g := &G{s: s, name: name, at: at, fns: fns, st: st, outside: outside}
	if !outside {
		if len(fns) == 0 {
			s.fail("%s: no function", name)
		}
		for _, f := range fns {
			s.need(need{kind: needFunc, fns: []string{f}})
		}
	}
	s.gs = append(s.gs, g)
	return g
}

// Main is a goroutine that runs from the start, the one calling the function
// the story is about.
func (s *Story) Main(name string, at Pt, fns ...string) *G {
	return s.addG(name, at, fns, running, false)
}

// Go is a goroutine started later (Spawn, WG.Go or Arrive).
func (s *Story) Go(name string, at Pt, fns ...string) *G {
	return s.addG(name, at, fns, hidden, false)
}

// Background marks g as running for the life of the process (e.g. a
// queue's consumer): it may still run or wait when the story ends.
func (g *G) Background() *G {
	g.background = true
	return g
}

// Outside is an actor outside the code shown, e.g. Ctrl-C or a browser.
func (s *Story) Outside(name string, at Pt) *G {
	return s.addG(name, at, nil, running, true)
}

func (g *G) bounds() rect   { return around(g.at, gW, gH) }
func (g *G) label() string  { return g.name }
func (g *G) String() string { return g.name }

func (g *G) handPos(i, n int) Pt {
	return Pt{g.at.X + (float64(i)-float64(n-1)/2)*(slotW+4), g.at.Y - gH/2 - tokH/2 - 4}
}

func (g *G) live(op string) {
	if g.st != running {
		g.s.fail("%s: %s, but it is %s", g.name, op, g.st)
	}
}

func (g *G) setStatus(st string) {
	for _, l := range strings.Split(st, "\n") {
		if len([]rune(l)) > maxStatus {
			g.s.fail("%s: status line %q longer than %d characters", g.name, l, maxStatus)
		}
	}
	g.status = st
}

func (g *G) req(n need) {
	if !g.outside {
		n.fns = g.fns
		g.s.need(n)
	}
}

func (g *G) start(by *G, how string) {
	if g.st != hidden {
		g.s.fail("%s started twice", g.name)
	}
	e := event{kind: evSpawn, to: g, label: how}
	if by != nil {
		e.from = by
	}
	g.s.ev(e)
	g.st = running
	g.setStatus("started")
}

// Spawn starts child with a go statement.
func (g *G) Spawn(child *G) {
	g.live("go")
	g.req(need{kind: needGo})
	how := "go"
	if g.s.inDo {
		how = "once.Do: go"
	}
	child.start(g, how)
	g.s.logf("%s: %s → %s starts", g, how, child)
}

// Arrive starts g from outside the code shown, e.g. net/http for a request.
func (g *G) Arrive(status string) {
	g.start(nil, "")
	g.setStatus(status)
	g.s.logf("%s starts (%s)", g, status)
}

// Busy says what g is doing.
func (g *G) Busy(status string) {
	g.live("busy")
	g.setStatus(status)
	g.s.logf("%s: %s", g, strings.ReplaceAll(status, "\n", " "))
}

// Calls claims that g's code calls each of texts, e.g. "lib.start".
func (g *G) Calls(texts ...string) {
	g.live("call")
	for _, t := range texts {
		g.req(need{kind: needCall, codes: []string{t}})
	}
}

// Exit ends g; a goroutine started by a WaitGroup counts it down.
func (g *G) Exit(status string) {
	g.live("exit")
	g.dropHand()
	e := event{kind: evExit, from: g, label: "returns"}
	if g.wg != nil {
		e.to, e.label, e.wg, e.n = g.wg.box, "done", g.wg, g.wg.n-1
	}
	g.s.ev(e)
	g.st = exited
	g.setStatus(status)
	g.s.logf("%s returns (%s)", g, status)
	if wg := g.wg; wg != nil {
		wg.n--
		wg.show()
		g.s.logf("  %s counter %d", wg.codes[0], wg.n)
		if wg.n == 0 {
			for len(wg.waiters) > 0 {
				w := wg.waiters[0]
				g.s.ev(event{kind: evWake, from: wg.box, to: w.g, label: "Wait returns"})
				g.s.logf("  %s: %s.Wait() returns", w.g, wg.codes[0])
				g.s.wake(w, nil, "wg.Wait returned")
			}
		}
	}
}

func (g *G) dropHand() {
	for len(g.hand) > 0 {
		g.hand[0].vanish()
	}
}

// --- values ---

// Tok is a value on its way: in a goroutine's hand, in a channel's buffer, or
// gone.
type Tok struct {
	label  string
	holder *G
	ch     *C
	gone   bool
	last   Pt

	// The current step: where it was at the start, whether it existed then,
	// whether it was created during the step, and the points it passed.
	start  Pt
	alive0 bool
	born   bool
	via    []Pt
}

func (s *Story) newTok(label string, at Pt) *Tok {
	if len([]rune(label)) > maxTok {
		s.fail("value %q longer than %d characters", label, maxTok)
	}
	t := &Tok{label: label, last: at, start: at, born: true}
	s.toks = append(s.toks, t)
	return t
}

// bornInHand creates a value in g's hand.
func (s *Story) bornInHand(g *G, label string) *Tok {
	t := s.newTok(label, Pt{})
	t.holder = g
	g.hand = append(g.hand, t)
	t.start = t.pos()
	t.last = t.start
	return t
}

func (t *Tok) pos() Pt {
	switch {
	case t.holder != nil:
		return t.holder.handPos(slices.Index(t.holder.hand, t), len(t.holder.hand))
	case t.ch != nil:
		return t.ch.slotPos(slices.Index(t.ch.buf, t))
	}
	return t.last
}

// move detaches t from where it is, passing through via.
func (t *Tok) move(via ...Pt) {
	p := t.pos()
	t.via = append(append(t.via, p), via...)
	t.last = p
	if len(via) > 0 {
		t.last = via[len(via)-1]
	}
	if h := t.holder; h != nil {
		h.hand = slices.DeleteFunc(h.hand, func(x *Tok) bool { return x == t })
		t.holder = nil
	}
	if c := t.ch; c != nil {
		c.buf = slices.DeleteFunc(c.buf, func(x *Tok) bool { return x == t })
		t.ch = nil
	}
}

func (t *Tok) vanish(via ...Pt) {
	t.move(via...)
	t.gone = true
}

// give puts t into g's hand; what g held before is used up.
func give(g *G, t *Tok) {
	g.dropHand()
	t.move()
	t.holder = g
	g.hand = append(g.hand, t)
}

// --- channels ---

// C is a channel.
type C struct {
	s       *Story
	name    string // as shown
	elem    string // element type
	slots   int    // buffer as drawn
	at      Pt
	dir     int      // +1: values flow left to right, -1: right to left
	codes   []string // how the code names it, e.g. "b.notify", "box.notify"
	capText string   // capacity expression in make, "" if unbuffered
	made    bool
	outside bool
	meta    string
	side    byte // where name and description go: 'a'bove, 'b'elow, 'l'eft, 'r'ight

	buf          []*Tok
	closed       bool
	sendq, recvq []*waiter
}

// Chan adds a channel with slots buffer slots (as drawn: a capacity
// len(observers) is drawn with as many slots as the story has observers).
func (s *Story) Chan(name, elem string, slots int, at Pt, dir int, codes ...string) *C {
	c := &C{s: s, name: name, elem: elem, slots: slots, at: at, dir: dir, codes: codes, side: 'a'}
	s.cs = append(s.cs, c)
	return c
}

// Made claims that fn makes the channel with capacity capExpr ("" for none).
func (c *C) Made(fn, capExpr string) *C {
	if (capExpr == "") != (c.slots == 0) {
		c.s.fail("channel %s: %d slots, capacity %q", c.name, c.slots, capExpr)
	}
	c.made, c.capText = true, capExpr
	c.s.need(need{kind: needMake, fns: []string{fn}, codes: c.codes, text: capExpr, elem: c.elem})
	return c
}

// Labels puts the channel's name and description above (the default),
// below, left or right of it: 'a', 'b', 'l' or 'r'.
func (c *C) Labels(side byte) *C {
	c.side = side
	return c
}

// Outside marks a channel made outside the code shown (a context's Done),
// described by meta.
func (c *C) Outside(meta string) *C {
	c.outside, c.meta = true, meta
	return c
}

func (c *C) describe() string {
	if c.outside {
		return c.meta
	}
	if c.capText == "" {
		return "chan " + c.elem + ", unbuffered"
	}
	d := "chan " + c.elem + ", cap " + c.capText
	if c.capText != fmt.Sprint(c.slots) {
		d += fmt.Sprintf(" = %d", c.slots)
	}
	return d
}

func (c *C) width() float64 {
	if c.slots == 0 {
		return unbufW
	}
	return float64(c.slots)*slotW + chPad
}

func (c *C) bounds() rect  { return around(c.at, c.width(), chH) }
func (c *C) label() string { return c.name }

// slotPos is the centre of buffer slot i, slot 0 being the next to leave.
func (c *C) slotPos(i int) Pt {
	if c.slots == 0 {
		return c.at
	}
	off := -c.width()/2 + chPad/2 + slotW/2 + float64(i)*slotW
	if c.dir > 0 {
		off = -off
	}
	return Pt{c.at.X + off, c.at.Y}
}

// Case is one case of a select.
type Case struct {
	c     *C
	send  bool
	label string
}

// SendCase is "case c <- label:".
func SendCase(c *C, label string) Case { return Case{c: c, send: true, label: label} }

// RecvCase is "case <-c:".
func RecvCase(c *C) Case { return Case{c: c} }

func (k Case) String() string {
	if k.send {
		return k.c.name + " <- " + k.label
	}
	return "<-" + k.c.name
}

// waiter is a blocked goroutine: on channel cases or on a WaitGroup.
type waiter struct {
	g     *G
	cases []Case
	toks  []*Tok // the value of each send case
	rng   bool   // a range loop
	wg    *WG
	mu    *Mutex
}

func (s *Story) need(n need) {
	k := fmt.Sprintf("%#v", n)
	if !s.seen[k] {
		s.seen[k] = true
		s.needs = append(s.needs, n)
	}
}

// Send is "c <- label".
func (g *G) Send(c *C, label string) {
	g.live("send")
	g.req(need{kind: needSend, codes: c.codes})
	g.sel([]Case{SendCase(c, label)}, false, false, c.name+" <- "+label)
}

// Recv is "<-c".
func (g *G) Recv(c *C) {
	g.live("receive")
	g.req(need{kind: needRecv, codes: c.codes})
	g.sel([]Case{RecvCase(c)}, false, false, "<-"+c.name)
}

// Range is one iteration of "for x := range c"; once c is closed and empty,
// the loop ends.
func (g *G) Range(c *C) {
	g.live("range")
	g.req(need{kind: needRange, codes: c.codes})
	g.sel([]Case{RecvCase(c)}, false, true, "range "+c.name)
}

// TrySend is "select { case c <- label: default: }" and reports whether the
// value was sent.
func (g *G) TrySend(c *C, label string) bool {
	g.live("try send")
	g.req(need{kind: needSelect, cases: []caseNeed{{send: true, codes: c.codes}}, dflt: true})
	return g.sel([]Case{SendCase(c, label)}, true, false, "select { "+c.name+" <- "+label+" | default }") >= 0
}

// Poll is "select { case <-c: default: }" and reports whether it received.
func (g *G) Poll(c *C) bool {
	g.live("poll")
	g.req(need{kind: needSelect, cases: []caseNeed{{codes: c.codes}}, dflt: true})
	return g.sel([]Case{RecvCase(c)}, true, false, "select { <-"+c.name+" | default }") >= 0
}

// Select is a select without default. Of several ready cases it takes the
// first listed (Go picks one at random; the story decides).
func (g *G) Select(cases ...Case) {
	g.live("select")
	n := need{kind: needSelect}
	var ks []string
	for _, k := range cases {
		n.cases = append(n.cases, caseNeed{send: k.send, codes: k.c.codes})
		ks = append(ks, k.String())
	}
	g.req(n)
	g.sel(cases, false, false, "select { "+strings.Join(ks, " | ")+" }")
}

// Close is "close(c)": every goroutine waiting to receive wakes up.
func (g *G) Close(c *C) {
	g.live("close")
	g.req(need{kind: needClose, codes: c.codes})
	if c.closed {
		g.s.fail("%s: close of closed channel %s", g.name, c.name)
	}
	if len(c.sendq) > 0 {
		g.s.fail("%s: close of %s while %s is blocked sending on it", g.name, c.name, c.sendq[0].g.name)
	}
	g.s.ev(event{kind: evClose, from: g, to: c, label: "close"})
	c.closed = true
	var woken []string
	for len(c.recvq) > 0 {
		w := c.recvq[0]
		g.s.zap(c, w.g)
		st := c.name + " closed"
		if w.rng {
			st = "range " + c.name + " ends"
		}
		woken = append(woken, w.g.name)
		g.s.wake(w, c, st)
	}
	if woken == nil {
		g.s.logf("%s: close(%s), nobody waits on it", g, c.name)
	} else {
		g.s.logf("%s: close(%s) → wakes %s", g, c.name, strings.Join(woken, ", "))
	}
}

func (c *C) sendReady() bool {
	if c.closed {
		c.s.fail("send on closed channel %s", c.name)
	}
	return len(c.recvq) > 0 || len(c.buf) < c.slots
}

func (c *C) recvReady() bool { return len(c.buf) > 0 || len(c.sendq) > 0 || c.closed }

// sel runs a select of cases for g (one case: a plain channel operation) and
// returns the case that ran, or -1 if g blocked or took the default. op is
// the operation in words, for the log.
func (g *G) sel(cases []Case, dflt, rng bool, op string) int {
	s := g.s
	g.dropHand() // what it held is dealt with
	w := &waiter{g: g, cases: cases, rng: rng, toks: make([]*Tok, len(cases))}
	for i, k := range cases {
		if k.send {
			w.toks[i] = s.bornInHand(g, k.label)
		}
	}
	for i, k := range cases {
		if k.send && k.c.sendReady() || !k.send && k.c.recvReady() {
			s.logf("%s: %s → %s", g, op, s.fire(w, i))
			return i
		}
	}
	if dflt {
		for _, t := range w.toks {
			if t != nil {
				t.vanish()
			}
		}
		if cases[0].send {
			g.setStatus("default: " + cases[0].c.name + " full,\nvalue dropped")
			s.ev(event{kind: evNote, from: g, label: "default: dropped"})
			s.logf("%s: %s → default: %s is full, the value is dropped", g, op, cases[0].c.name)
		} else {
			g.setStatus("default: " + cases[0].c.name + " not ready")
			s.ev(event{kind: evNote, from: g, label: "default: not ready"})
			s.logf("%s: %s → default: not ready", g, op)
		}
		return -1
	}
	for _, k := range cases {
		if k.send {
			k.c.sendq = append(k.c.sendq, w)
		} else {
			k.c.recvq = append(k.c.recvq, w)
		}
	}
	g.w, g.st = w, blocked
	switch {
	case len(cases) > 1:
		var ks []string
		for _, k := range cases {
			ks = append(ks, k.String())
		}
		g.setStatus(selectStatus(ks))
	case rng:
		g.setStatus("blocked: range " + cases[0].c.name)
	default:
		g.setStatus("blocked: " + cases[0].String())
	}
	s.logf("%s: %s → blocks", g, op)
	return -1
}

// selectStatus is the status of a goroutine blocked in a select with cases
// ks: "blocked in select:" and the cases if they fit on one line, else the
// cases alone, as "select …", over two lines.
func selectStatus(ks []string) string {
	if one := strings.Join(ks, " | "); len([]rune(one)) <= maxStatus {
		return "blocked in select:\n" + one
	}
	lines := []string{"select " + ks[0]}
	for _, k := range ks[1:] {
		last := &lines[len(lines)-1]
		if len([]rune(*last+" | "+k)) <= maxStatus {
			*last += " | " + k
		} else {
			*last += " |"
			lines = append(lines, k)
		}
	}
	return strings.Join(lines, "\n")
}

// fire runs case i of w, which is ready, for the running goroutine w.g, and
// says what happened.
func (s *Story) fire(w *waiter, i int) string {
	g, k := w.g, w.cases[i]
	c := k.c
	var what string
	if k.send {
		t := w.toks[i]
		if len(c.recvq) > 0 {
			r := c.recvq[0]
			s.ev(event{kind: evHandoff, from: g, via: c, to: r.g, label: t.label})
			t.move(c.at)
			s.wake(r, c, "got "+t.label)
			give(r.g, t)
			what = "hands " + t.label + " to " + r.g.name + ", who wakes up"
		} else {
			s.ev(event{kind: evSend, from: g, to: c, label: t.label, tok: t})
			t.move()
			t.ch = c
			c.buf = append(c.buf, t)
			what = t.label + " waits in the buffer of " + c.name
		}
		g.setStatus("sent " + t.label)
	} else {
		switch {
		case len(c.buf) > 0:
			t := c.buf[0]
			s.ev(event{kind: evRecv, from: c, to: g, label: t.label, tok: t})
			give(g, t)
			what = "takes " + t.label + " from the buffer"
			if len(c.sendq) > 0 {
				sw := c.sendq[0]
				st := sw.tokOn(c)
				s.ev(event{kind: evSend, from: sw.g, to: c, label: st.label, tok: st})
				st.move()
				st.ch = c
				c.buf = append(c.buf, st)
				s.wake(sw, c, "sent "+st.label)
				what += "; " + sw.g.name + "'s " + st.label + " moves in, and it wakes up"
			}
			g.setStatus("got " + t.label)
		case len(c.sendq) > 0:
			sw := c.sendq[0]
			t := sw.tokOn(c)
			s.ev(event{kind: evHandoff, from: sw.g, via: c, to: g, label: t.label})
			t.move(c.at)
			s.wake(sw, c, "sent "+t.label)
			give(g, t)
			g.setStatus("got " + t.label)
			what = "takes " + t.label + " from " + sw.g.name + ", who wakes up"
		default: // closed
			s.zap(c, g)
			if w.rng {
				g.setStatus("range " + c.name + " ends")
				what = c.name + " is closed: the loop ends"
			} else {
				g.setStatus(c.name + " closed")
				what = c.name + " is closed: ready at once"
			}
		}
	}
	for _, t := range w.toks {
		if t != nil && t.holder == g {
			t.vanish()
		}
	}
	return what
}

func (w *waiter) tokOn(c *C) *Tok {
	for i, k := range w.cases {
		if k.c == c && k.send {
			return w.toks[i]
		}
	}
	panic("no send case on " + c.name)
}

// wake unblocks w (through channel c, or its WaitGroup if c is nil).
func (s *Story) wake(w *waiter, c *C, status string) {
	for _, k := range w.cases {
		k.c.sendq = slices.DeleteFunc(k.c.sendq, func(x *waiter) bool { return x == w })
		k.c.recvq = slices.DeleteFunc(k.c.recvq, func(x *waiter) bool { return x == w })
	}
	if w.wg != nil {
		w.wg.waiters = slices.DeleteFunc(w.wg.waiters, func(x *waiter) bool { return x == w })
	}
	if w.mu != nil {
		w.mu.waiters = slices.DeleteFunc(w.mu.waiters, func(x *waiter) bool { return x == w })
	}
	for i, t := range w.toks {
		if t != nil && t.holder == w.g && (c == nil || w.cases[i].c != c) {
			t.vanish()
		}
	}
	w.g.w, w.g.st = nil, running
	w.g.setStatus(status)
}

// zap shows a receive from the closed channel c reaching g.
func (s *Story) zap(c *C, g *G) {
	s.ev(event{kind: evClosed, from: c, to: g, label: "closed"})
	t := s.newTok("closed", c.at)
	t.vanish(g.handPos(0, 1))
}

// --- other state ---

// Box shows state that is not a channel: a mailbox's queue, a map, fields.
type Box struct {
	title string
	r     rect
	lines []string
	wg    bool // a WaitGroup: lines[0] is its counter
}

// Box adds a box of w×h around at.
func (s *Story) Box(title string, at Pt, w, h float64, lines ...string) *Box {
	b := &Box{title: title, r: around(at, w, h), lines: lines}
	s.boxes = append(s.boxes, b)
	return b
}

func (b *Box) bounds() rect  { return b.r }
func (b *Box) label() string { return b.title }

// Set changes what the box shows.
func (b *Box) Set(lines ...string) { b.lines = lines }

// Give sends a value labelled label from g into b by calling call (e.g.
// "boxes[id].put"), and sets b to lines.
func (g *G) Give(b *Box, call, label string, lines ...string) {
	g.live("give")
	g.req(need{kind: needCall, codes: []string{call}})
	g.s.ev(event{kind: evGive, from: g, to: b, label: label})
	g.s.bornInHand(g, label).vanish(b.r.center())
	b.Set(lines...)
	g.s.logf("%s: %s(%s) → %s: %s", g, call, label, b.title, strings.Join(lines, ", "))
}

// Take moves a value labelled label from b into g's hand by calling call,
// and sets b to lines.
func (g *G) Take(b *Box, call, label string, lines ...string) {
	g.live("take")
	g.req(need{kind: needCall, codes: []string{call}})
	g.s.ev(event{kind: evTake, from: b, to: g, label: label})
	give(g, g.s.newTok(label, b.r.center()))
	b.Set(lines...)
	g.s.logf("%s: %s() → %s", g, call, label)
}

// Store puts what g holds into b and sets b to lines.
func (g *G) Store(b *Box, lines ...string) {
	g.live("store")
	for len(g.hand) > 0 {
		g.hand[0].vanish(b.r.center())
	}
	b.Set(lines...)
	g.s.logf("%s: %s: %s", g, b.title, strings.Join(lines, ", "))
}

// Write puts a value labelled label from g into b by assigning to lhs (e.g.
// "hyps[i]"), and sets b to lines.
func (g *G) Write(b *Box, lhs, label string, lines ...string) {
	g.live("write")
	g.req(need{kind: needAssign, codes: []string{lhs}})
	g.s.ev(event{kind: evGive, from: g, to: b, label: label})
	g.s.bornInHand(g, label).vanish(b.r.center())
	b.Set(lines...)
	g.s.logf("%s: %s = %s → %s: %s", g, lhs, label, b.title, strings.Join(lines, ", "))
}

// Drop throws away what g holds.
func (g *G) Drop(status string) {
	g.live("drop")
	g.dropHand()
	g.setStatus(status)
	g.s.ev(event{kind: evNote, from: g, label: status})
	g.s.logf("%s: %s", g, status)
}

// WG is a sync.WaitGroup.
type WG struct {
	box     *Box
	codes   []string
	n       int
	waiters []*waiter
}

// WaitGroup adds a WaitGroup that the code calls codes (e.g. "wg").
func (s *Story) WaitGroup(at Pt, codes ...string) *WG {
	wg := &WG{box: s.Box(codes[0]+" (sync.WaitGroup)", at, 190, 56), codes: codes}
	wg.box.wg = true
	wg.show()
	return wg
}

func (wg *WG) show() { wg.box.Set(strconv.Itoa(wg.n)) }

func (wg *WG) calls(method string) need {
	n := need{kind: needCall}
	for _, c := range wg.codes {
		n.codes = append(n.codes, c+"."+method)
	}
	return n
}

// Go is "wg.Go(child)" called by parent.
func (wg *WG) Go(parent, child *G) {
	parent.live("wg.Go")
	parent.req(wg.calls("Go"))
	child.wg = wg
	wg.n++
	wg.show()
	child.start(parent, wg.codes[0]+".Go")
	e := parent.s.rows[len(parent.s.rows)-1].ev
	e.wg, e.n = wg, wg.n
	parent.s.logf("%s: %s.Go → %s starts, counter %d", parent, wg.codes[0], child, wg.n)
}

// Wait is "wg.Wait()".
func (g *G) Wait(wg *WG) {
	g.live("wg.Wait")
	g.req(wg.calls("Wait"))
	if wg.n == 0 {
		g.setStatus("wg.Wait returned")
		g.s.logf("%s: %s.Wait() → returns at once, counter 0", g, wg.codes[0])
		return
	}
	w := &waiter{g: g, wg: wg}
	wg.waiters = append(wg.waiters, w)
	g.w, g.st = w, blocked
	g.setStatus("blocked: wg.Wait()")
	g.s.logf("%s: %s.Wait() → blocks, counter %d", g, wg.codes[0], wg.n)
}

// Mutex is a sync.Mutex. Its box shows who holds it and who waits.
type Mutex struct {
	box     *Box
	codes   []string
	holder  *G
	waiters []*waiter
}

// Mutex adds a sync.Mutex that the code calls codes (e.g. "pm.mu").
func (s *Story) Mutex(at Pt, codes ...string) *Mutex {
	m := &Mutex{box: s.Box(codes[0]+" (sync.Mutex)", at, 190, 56), codes: codes}
	m.show()
	return m
}

func (m *Mutex) show() {
	if m.holder == nil {
		m.box.Set("free")
		return
	}
	l := []string{"held by " + m.holder.name}
	if len(m.waiters) > 0 {
		var ws []string
		for _, w := range m.waiters {
			ws = append(ws, w.g.name)
		}
		l = append(l, "waiting: "+strings.Join(ws, ", "))
	}
	m.box.Set(l...)
}

func (m *Mutex) calls(method string) need {
	n := need{kind: needCall}
	for _, c := range m.codes {
		n.codes = append(n.codes, c+"."+method)
	}
	return n
}

// Lock is "mu.Lock()": it blocks while another goroutine holds m.
func (g *G) Lock(m *Mutex) {
	g.live("Lock")
	g.req(m.calls("Lock"))
	name := m.codes[0]
	if m.holder == g {
		g.s.fail("%s: Lock of %s, which it holds already", g.name, name)
	}
	if m.holder == nil {
		g.s.ev(event{kind: evLock, from: g, to: m.box, label: "Lock"})
		m.holder = g
		m.show()
		g.s.logf("%s: %s.Lock() → holds it", g, name)
		return
	}
	w := &waiter{g: g, mu: m}
	m.waiters = append(m.waiters, w)
	g.w, g.st = w, blocked
	g.setStatus("blocked: " + name + ".Lock()")
	m.show()
	g.s.logf("%s: %s.Lock() → blocks, %s holds it", g, name, m.holder)
}

// Unlock is "mu.Unlock()": the first waiter, if any, gets the lock.
func (g *G) Unlock(m *Mutex) {
	g.live("Unlock")
	g.req(m.calls("Unlock"))
	name := m.codes[0]
	if m.holder != g {
		g.s.fail("%s: Unlock of %s, which it does not hold", g.name, name)
	}
	g.s.ev(event{kind: evUnlock, from: g, to: m.box, label: "Unlock"})
	m.holder = nil
	if len(m.waiters) > 0 {
		w := m.waiters[0]
		g.s.ev(event{kind: evWake, from: m.box, to: w.g, label: "Lock returns"})
		g.s.wake(w, nil, "holds "+name)
		m.holder = w.g
		g.s.logf("%s: %s.Unlock() → %s gets the lock and wakes up", g, name, w.g)
	} else {
		g.s.logf("%s: %s.Unlock()", g, name)
	}
	m.show()
}

// Once is a sync.Once.
type Once struct {
	box   *Box
	codes []string
	done  bool
}

// Once adds a sync.Once that the code calls codes (e.g. "e.once").
func (s *Story) Once(at Pt, codes ...string) *Once {
	return &Once{box: s.Box(codes[0]+" (sync.Once)", at, 170, 44, "not done"), codes: codes}
}

// Do is "once.Do(f)": f runs on the first call only.
func (g *G) Do(o *Once, f func()) {
	g.live("once.Do")
	n := need{kind: needCall}
	for _, c := range o.codes {
		n.codes = append(n.codes, c+".Do")
	}
	g.req(n)
	if o.done {
		g.setStatus("once.Do: already done")
		g.s.logf("%s: %s.Do → already done, does nothing", g, o.codes[0])
		return
	}
	o.done = true
	o.box.Set("done")
	g.s.logf("%s: %s.Do → first call, runs the function", g, o.codes[0])
	g.s.inDo = true
	f()
	g.s.inDo = false
}

// --- frames ---

type frame struct {
	caption string
	ops     []string
	gs      []gFrame
	closed  []bool
	boxes   [][]string
	toks    []tokFrame
	waits   []waitEdge
}

type gFrame struct {
	st     state
	status string
}

type tokFrame struct {
	born, alive0, alive1 bool
	path                 []Pt
}

type waitEdge struct {
	g  int
	to rect
}

func (f frame) moves() bool {
	for _, t := range f.toks {
		if t.born || t.alive0 != t.alive1 || len(t.path) > 1 {
			return true
		}
	}
	return false
}

func (s *Story) capture() frame {
	var f frame
	f.gs = s.states()
	for i, g := range s.gs {
		if g.w == nil {
			continue
		}
		if g.w.wg != nil {
			f.waits = append(f.waits, waitEdge{i, g.w.wg.box.r})
		}
		if g.w.mu != nil {
			f.waits = append(f.waits, waitEdge{i, g.w.mu.box.r})
		}
		for _, k := range g.w.cases {
			e := waitEdge{i, k.c.bounds()}
			if !slices.Contains(f.waits, e) {
				f.waits = append(f.waits, e)
			}
		}
	}
	for _, c := range s.cs {
		f.closed = append(f.closed, c.closed)
	}
	for _, b := range s.boxes {
		f.boxes = append(f.boxes, slices.Clone(b.lines))
	}
	for _, t := range s.toks {
		path := append([]Pt{t.start}, t.via...)
		if !t.gone {
			path = append(path, t.pos())
		}
		path = slices.Compact(path)
		f.toks = append(f.toks, tokFrame{born: t.born, alive0: t.alive0, alive1: !t.gone, path: path})
	}
	return f
}
