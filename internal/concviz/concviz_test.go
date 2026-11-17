package concviz

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

var update = flag.Bool("update", false, "rewrite the sequence diagrams, player.html and the blocks of concurrency.md")

const root = "../.."

// golden compares got with the file, or with -update writes it.
func golden(t *testing.T, file string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	} else if old, err := os.ReadFile(file); err != nil {
		t.Error(err)
	} else if !bytes.Equal(old, got) {
		t.Errorf("%s is not what the stories draw; rewrite it: go test ./internal/concviz -update", file)
	}
}

// TestOutputsUpToDate checks the files generated from the stories: a
// sequence diagram per story, the player, and the blocks in
// concurrency.md. With -update it rewrites them.
func TestOutputsUpToDate(t *testing.T) {
	dir := filepath.Join(root, "architecture", "concurrency")
	stories := Stories()
	for _, s := range stories {
		golden(t, filepath.Join(dir, s.File), s.Sequence())
	}
	golden(t, filepath.Join(dir, "player.html"), Player(stories))

	mdFile := filepath.Join(root, "architecture", "concurrency.md")
	md, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UpdateDoc(string(md), stories)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, mdFile, []byte(got))
}

// TestStoriesMatchCode checks every claim the stories make about the code:
// functions, how channels are made, sends, receives, selects, closes, calls.
func TestStoriesMatchCode(t *testing.T) {
	src, err := LoadSource(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Stories() {
		if len(s.needs) == 0 {
			t.Errorf("%s: claims nothing about the code", s.File)
		}
		for _, err := range s.Check(src) {
			t.Error(err)
		}
	}
}

// TestCheckFindsDrift makes sure the check notices when code and story part.
func TestCheckFindsDrift(t *testing.T) {
	src, err := LoadSource(fstest.MapFS{"p/p.go": {Data: []byte(`package p

func Run(n int) {
	out := make(chan int, n)
	quit := make(chan struct{})
	go func() {
		select {
		case out <- 1:
		case <-quit:
		}
	}()
	close(quit)
	<-out
}
`)}})
	if err != nil {
		t.Fatal(err)
	}
	story := func(capExpr string, closeOut bool, sel ...func(*C, *C) Case) *Story {
		s := newStory("test.svg", "test", "p/p.go", 400, 300)
		main := s.Main("main", Pt{50, 50}, "p.Run")
		g := s.Go("g", Pt{200, 50}, "p.Run")
		out := s.Chan("out", "int", 1, Pt{100, 100}, 1, "out").Made("p.Run", capExpr)
		quit := s.Chan("quit", "struct{}", 0, Pt{100, 200}, 1, "quit").Made("p.Run", "")
		s.Step("")
		main.Spawn(g)
		var cases []Case
		for _, f := range sel {
			cases = append(cases, f(out, quit))
		}
		g.Select(cases...)
		if closeOut {
			main.Recv(out)
			main.Close(out)
		} else {
			main.Close(quit)
			main.Recv(out)
		}
		return s
	}
	send := func(out, _ *C) Case { return SendCase(out, "1") }
	recvQuit := func(_, quit *C) Case { return RecvCase(quit) }
	recvOut := func(out, _ *C) Case { return RecvCase(out) }
	for _, tc := range []struct {
		name  string
		story *Story
		errs  int
	}{
		{"matches", story("n", false, send, recvQuit), 0},
		{"capacity", story("1", false, send, recvQuit), 1},
		{"select case missing", story("n", false, send), 1},
		{"select case wrong", story("n", false, send, recvOut), 1},
		{"close of other channel", story("n", true, send, recvQuit), 1},
	} {
		if errs := tc.story.Check(src); len(errs) != tc.errs {
			t.Errorf("%s: %d errors, want %d: %v", tc.name, len(errs), tc.errs, errs)
		}
	}
}

// TestCheckFindsDriftVarsAndWrites covers channels made in a package-level
// var and results written by index.
func TestCheckFindsDriftVarsAndWrites(t *testing.T) {
	src, err := LoadSource(fstest.MapFS{"q/q.go": {Data: []byte(`package q

var slots = make(chan struct{}, 4)

func Run(res []int) {
	slots <- struct{}{}
	res[0] = 1
	<-slots
}
`)}})
	if err != nil {
		t.Fatal(err)
	}
	story := func(capExpr, lhs string) *Story {
		s := newStory("test.svg", "test", "q/q.go", 400, 300)
		g := s.Main("g", Pt{50, 50}, "q.Run")
		c := s.Chan("slots", "struct{}", 1, Pt{100, 100}, 1, "slots").Made("q.slots", capExpr)
		b := s.Box("res", Pt{200, 200}, 100, 40)
		s.Step("")
		g.Send(c, "{}")
		g.Write(b, lhs, "1")
		g.Recv(c)
		return s
	}
	for _, tc := range []struct {
		name  string
		story *Story
		errs  int
	}{
		{"matches", story("4", "res[0]"), 0},
		{"capacity", story("8", "res[0]"), 1},
		{"write elsewhere", story("4", "res[1]"), 1},
	} {
		if errs := tc.story.Check(src); len(errs) != tc.errs {
			t.Errorf("%s: %d errors, want %d: %v", tc.name, len(errs), tc.errs, errs)
		}
	}
}

// TestChannelModel checks the model's semantics on small scripts.
func TestChannelModel(t *testing.T) {
	scene := func() (*Story, *G, *G, *G) {
		s := newStory("test.svg", "test", "", 400, 300)
		a := s.Main("a", Pt{50, 50}, "p.A")
		b := s.Main("b", Pt{150, 50}, "p.B")
		c := s.Main("c", Pt{250, 50}, "p.C")
		s.Step("")
		return s, a, b, c
	}
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		f()
	}

	s, a, b, _ := scene()
	ch := s.Chan("ch", "int", 1, Pt{100, 100}, 1, "ch")
	a.Send(ch, "1") // buffered: does not block
	a.Send(ch, "2") // full: blocks
	if a.st != blocked || len(ch.buf) != 1 {
		t.Errorf("full buffer: a is %s, buffer %d", a.st, len(ch.buf))
	}
	b.Recv(ch) // takes 1, a's 2 moves into the buffer
	if a.st != running || len(ch.buf) != 1 || ch.buf[0].label != "2" || b.hand[0].label != "1" {
		t.Errorf("receive from full buffer: a is %s, buffer %v", a.st, ch.buf)
	}
	if b.TrySend(ch, "3") {
		t.Error("TrySend on a full buffer sent")
	}

	s, a, b, c := scene()
	un := s.Chan("un", "int", 0, Pt{100, 100}, 1, "un")
	a.Recv(un)
	b.Range(un)
	c.Close(un)
	if a.st != running || b.st != running || b.status != "range un ends" {
		t.Errorf("close: a %s, b %s %q", a.st, b.st, b.status)
	}
	mustPanic("send on closed", func() { a.Send(un, "x") })
	mustPanic("close of closed", func() { a.Close(un) })

	s, a, b, c = scene()
	x := s.Chan("x", "int", 0, Pt{100, 100}, 1, "x")
	y := s.Chan("y", "int", 0, Pt{100, 200}, 1, "y")
	a.Select(SendCase(x, "1"), RecvCase(y))
	b.Send(y, "2") // wakes a through y; a's send on x is withdrawn
	if a.st != running || len(x.sendq) != 0 || a.hand[0].label != "2" {
		t.Errorf("select: a %s, x senders %d", a.st, len(x.sendq))
	}
	mustPanic("blocked goroutine acts", func() { c.Recv(x); c.Send(y, "z") })

	s, a, b, _ = scene()
	wg := s.WaitGroup(Pt{100, 100}, "wg")
	g := s.Go("g", Pt{200, 200}, "p.G")
	wg.Go(a, g)
	b.Wait(wg)
	if b.st != blocked {
		t.Errorf("wg.Wait with counter 1: b %s", b.st)
	}
	g.Exit("done")
	if b.st != running {
		t.Errorf("wg.Wait after the last Exit: b %s", b.st)
	}

	s, a, b, c = scene()
	mu := s.Mutex(Pt{100, 100}, "mu")
	a.Lock(mu)
	b.Lock(mu) // held by a: blocks
	c.Lock(mu)
	if b.st != blocked || c.st != blocked {
		t.Errorf("Lock of a held mutex: b %s, c %s", b.st, c.st)
	}
	a.Unlock(mu) // the first waiter gets it
	if b.st != running || c.st != blocked || mu.holder != b {
		t.Errorf("Unlock: b %s, c %s, holder %v", b.st, c.st, mu.holder)
	}
	mustPanic("Unlock by another goroutine", func() { a.Unlock(mu) })
	mustPanic("Lock twice", func() { b.Lock(mu) })
}
