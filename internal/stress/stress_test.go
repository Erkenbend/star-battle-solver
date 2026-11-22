package stress

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/board"
	"starbattle/internal/collection"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
	"starbattle/internal/web"
)

func solve(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error) {
	res := solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true, Done: ctx.Done()})
	if res.Status == solver.Cancelled {
		return res, ctx.Err()
	}
	return res, nil
}

// server serves the site as serve does (budget, library), in this process.
func server(t *testing.T) (*httptest.Server, Config) {
	t.Helper()
	b := web.NewBudget(2)
	motifLib, err := web.LoadMotifs(starbattle.Designs, starbattle.DesignsDir, b.Limit(solve))
	if err != nil {
		t.Fatal(err)
	}
	hardLib, err := web.LoadLibrary(starbattle.Hard, starbattle.HardDir, b.Background(solve))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(web.Compress(web.Handler(b.Limit(solve), nil, nil, []*web.Library{motifLib, hardLib}, nil)))
	t.Cleanup(srv.Close)
	hard, err := collection.Load(starbattle.Hard, starbattle.HardDir)
	if err != nil {
		t.Fatal(err)
	}
	motifs, err := collection.Load(starbattle.Designs, starbattle.DesignsDir)
	if err != nil {
		t.Fatal(err)
	}
	return srv, Config{URL: srv.URL, PID: os.Getpid(), Hard: hard, Motifs: motifs, HeavyN: 10}
}

// TestScenarios runs every scenario against a server in this process, the
// crawl over a few small puzzles only; abandon takes 8 s and is left out
// with -short.
func TestScenarios(t *testing.T) {
	_, c := server(t)
	var small []collection.Entry
	for _, e := range c.Hard {
		if e.Puzzle.N <= 7 {
			small = append(small, e)
		}
	}
	wants := map[string][]string{
		"tutorial": {"200 × /anleitung", "server CPU"},
		"reload":   {"10 reloads", "8 players open a link at once", "that link once more"},
		"abandon":  {"12 visitors gone", "+8s: server CPU since they left"},
		"flood":    {"12 pastes at once", "latency: n=12"},
		"crawl":    {fmt.Sprint("crawler's pages: n=", len(small)), "every library solve done", "motif pages during the crawl: n=3"},
		"pages":    {"new 8×8, gzip false", "KB sent (none)", "tutorial, gzip true", "KB sent (gzip)", "latency: n=800 median"},
	}
	for _, s := range Scenarios {
		if s.Name == "abandon" && testing.Short() {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			var out strings.Builder
			cc := c
			cc.Out = &out
			if s.Name == "crawl" || s.Name == "pages" {
				cc.Hard = small
			}
			if err := Run(cc, s); err != nil {
				t.Fatalf("%v\n%s", err, out.String())
			}
			for _, want := range append(wants[s.Name], "== "+s.Name) {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in\n%s", want, out.String())
				}
			}
		})
	}
}

// TestFailures: requests that fail make Run fail and are named.
func TestFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "kaputt", http.StatusInternalServerError)
	}))
	defer srv.Close()
	s, _ := Find("tutorial")
	var out strings.Builder
	err := Run(Config{URL: srv.URL, Out: &out}, s)
	if err == nil || !strings.Contains(err.Error(), "200 requests failed") || !strings.Contains(out.String(), "FAILED /anleitung: status 500") {
		t.Fatalf("got %v\n%.300s", err, out.String())
	}
	if !strings.Contains(out.String(), "RSS n/a") {
		t.Errorf("without a PID, memory should be n/a:\n%.300s", out.String())
	}
}
