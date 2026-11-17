package concviz

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

var update = flag.Bool("update", false, "rewrite the animations in architecture/concurrency/")

const root = "../.."

// TestAnimationsUpToDate checks that the SVGs in architecture/concurrency/
// are what the stories draw. With -update it rewrites them.
func TestAnimationsUpToDate(t *testing.T) {
	for _, s := range Stories() {
		file := filepath.Join(root, "architecture", "concurrency", s.File)
		svg := s.SVG()
		if *update {
			if err := os.WriteFile(file, svg, 0o644); err != nil {
				t.Fatal(err)
			}
		} else if old, err := os.ReadFile(file); err != nil {
			t.Error(err)
		} else if !bytes.Equal(old, svg) {
			t.Errorf("%s is not what the story draws; rewrite it: go test ./internal/concviz -update", s.File)
		}
	}
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

// TestDocShowsAnimations checks that concurrency.md shows every animation.
func TestDocShowsAnimations(t *testing.T) {
	md, err := os.ReadFile(filepath.Join(root, "architecture", "concurrency.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Stories() {
		if !strings.Contains(string(md), "](concurrency/"+s.File+")") {
			t.Errorf("concurrency.md does not show concurrency/%s", s.File)
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
}
