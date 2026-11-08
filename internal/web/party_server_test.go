package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// postJSON posts body as JSON to h and decodes the reply into out (may be nil).
func postJSON(t *testing.T, h http.Handler, path string, body any, out any) int {
	t.Helper()
	data, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(string(data))))
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: %v: %s", path, err, rec.Body)
		}
	}
	return rec.Code
}

func TestPartyServer(t *testing.T) {
	pm, clk := newTestParties(t)
	h := Handler(testSolve, nil, pm)

	if rec := get(h, "/party"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="create"`) {
		t.Fatalf("start page: %d", rec.Code)
	}
	if rec := get(Handler(testSolve, nil, nil), "/party"); rec.Code != http.StatusNotFound {
		t.Fatalf("party mode without parties: %d", rec.Code)
	}

	// Creating redirects to the lobby.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/party", strings.NewReader(url.Values{"n": {"5", "x"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")
	m := regexp.MustCompile(`^/party/([A-Z]{4})$`).FindStringSubmatch(loc)
	if rec.Code != http.StatusSeeOther || m == nil {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	code := m[1]
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/party", strings.NewReader("n=99"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create without valid sizes: %d", rec.Code)
	}

	if rec := get(h, loc); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("lobby: %d", rec.Code)
	}
	if rec := get(h, "/party/"+strings.ToLower(code)); rec.Code != http.StatusFound || rec.Header().Get("Location") != loc {
		t.Fatalf("lower-case code: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(h, "/party/QQQQ"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown party: %d", rec.Code)
	}

	// Join, ready: the round starts.
	var joined struct {
		Token    string   `json:"token"`
		ID       string   `json:"id"`
		Snapshot Snapshot `json:"snapshot"`
	}
	if c := postJSON(t, h, loc+"/join", map[string]any{"name": "Anna"}, &joined); c != http.StatusOK || joined.Token == "" || joined.Snapshot.You != joined.ID {
		t.Fatalf("join: %d %+v", c, joined)
	}
	var errReply struct{ Error string }
	if c := postJSON(t, h, loc+"/ready", map[string]any{"token": "bogus", "ready": true}, &errReply); c != http.StatusForbidden || errReply.Error == "" {
		t.Fatalf("ready with a bad token: %d %+v", c, errReply)
	}
	if c := postJSON(t, h, loc+"/dance", map[string]any{}, nil); c != http.StatusNotFound {
		t.Fatalf("unknown action: %d", c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", loc+"/ready", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON: %d", rec.Code)
	}
	if rec := get(h, loc+"/round/1"); rec.Code != http.StatusNotFound {
		t.Fatalf("round page before the round: %d", rec.Code)
	}
	waitPrepared(t, pm, code)
	var ready struct{ Snapshot Snapshot }
	if c := postJSON(t, h, loc+"/ready", map[string]any{"token": joined.Token, "ready": true}, &ready); c != http.StatusOK || ready.Snapshot.Phase != PhasePlaying {
		t.Fatalf("ready: %d %+v", c, ready.Snapshot)
	}

	// The round's page carries the party; the next round's does not exist yet.
	rec = get(h, loc+"/round/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("round page: %d", rec.Code)
	}
	if d := pageData(t, rec.Body.String()); d.Party == nil || d.Party.Code != code || d.Party.Round != 1 || !strings.Contains(rec.Body.String(), "const Party =") {
		t.Fatalf("round page data: %+v", d.Party)
	}
	if strings.Contains(get(h, "/?n=5&seed=1").Body.String(), "const Party =") {
		t.Fatal("solo page has the party script")
	}
	for _, p := range []string{"/round/2", "/round/x"} {
		if rec := get(h, loc+p); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", p, rec.Code)
		}
	}

	clk.add(partyCountdown)
	if c := postJSON(t, h, loc+"/progress", map[string]any{"token": joined.Token, "round": 1, "count": 2, "hints": 1}, nil); c != http.StatusOK {
		t.Fatalf("progress: %d", c)
	}
	if c := postJSON(t, h, loc+"/solve", map[string]any{"token": joined.Token, "round": 1, "stars": []int{0, 0, 0, 0, 0}}, nil); c != http.StatusBadRequest {
		t.Fatalf("wrong solution: %d", c)
	}
	var solved struct {
		Result   Result
		Snapshot Snapshot
	}
	sol := testPuzzle(t, 5).Solution
	if c := postJSON(t, h, loc+"/solve", map[string]any{"token": joined.Token, "round": 1, "stars": sol}, &solved); c != http.StatusOK || !solved.Result.Solved || solved.Result.Hints != 1 || solved.Snapshot.Phase != PhaseResults {
		t.Fatalf("solve: %d %+v", c, solved)
	}
	var left struct{ Snapshot Snapshot }
	if c := postJSON(t, h, loc+"/leave", map[string]any{"token": joined.Token}, &left); c != http.StatusOK || left.Snapshot.You != "" || len(left.Snapshot.Players) != 0 {
		t.Fatalf("leave: %d %+v", c, left.Snapshot)
	}
}

func TestPartyEvents(t *testing.T) {
	pm, _ := newTestParties(t)
	srv := httptest.NewServer(Handler(testSolve, nil, pm))
	defer srv.Close()
	code, err := pm.Create([]int{5}, "")
	must(t, err)
	tok := mustJoin(t, pm, code, "Anna")

	if res, err := http.Get(srv.URL + "/party/QQQQ/events"); err != nil || res.StatusCode != http.StatusNotFound {
		t.Fatalf("events of an unknown party: %v %v", res.StatusCode, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/party/"+code+"/events?token="+tok, nil)
	res, err := http.DefaultClient.Do(req)
	must(t, err)
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(res.Body)
	next := func() Snapshot {
		t.Helper()
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var s Snapshot
				must(t, json.Unmarshal([]byte(data), &s))
				return s
			}
		}
		t.Fatalf("stream ended: %v", sc.Err())
		return Snapshot{}
	}
	if s := next(); s.Code != code || s.You == "" || len(s.Players) != 1 {
		t.Fatalf("first event %+v", s)
	}
	mustJoin(t, pm, code, "Ben")
	for s := next(); len(s.Players) != 2; s = next() {
	}

	cancel()
	waitFor(t, func() bool {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		p := pm.parties[code]
		return len(p.subs) == 0 && p.players[0].conns == 0
	})
}

func TestCoopServer(t *testing.T) {
	pm, clk := newTestParties(t)
	h := Handler(testSolve, nil, pm)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/party", strings.NewReader("n=5&mode=coop"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")
	code := strings.TrimPrefix(loc, "/party/")
	if rec.Code != http.StatusSeeOther || snap(t, pm, code, "").Mode != ModeCoop {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	waitPrepared(t, pm, code)
	var joined struct{ Token string }
	postJSON(t, h, loc+"/join", map[string]any{"name": "Anna"}, &joined)
	postJSON(t, h, loc+"/ready", map[string]any{"token": joined.Token, "ready": true}, nil)
	if d := pageData(t, get(h, loc+"/round/1").Body.String()); d.Party == nil || d.Party.Mode != ModeCoop {
		t.Fatalf("round page data: %+v", d.Party)
	}
	clk.add(partyCountdown)

	var marked struct {
		Rejected []int
		Snapshot Snapshot
	}
	ops := [][4]int{{1, 0, 0, 1}, {2, 0, 2, 0}}
	if c := postJSON(t, h, loc+"/mark", map[string]any{"token": joined.Token, "round": 1, "ops": ops}, &marked); c != http.StatusOK ||
		len(marked.Rejected) != 1 || marked.Rejected[0] != 2 || marked.Snapshot.Coop == nil || marked.Snapshot.Coop.Board[0] != '1' || marked.Snapshot.Coop.Acked != 2 {
		t.Fatalf("mark: %d %+v %+v", c, marked, marked.Snapshot.Coop)
	}
	// A whole board's worth of operations fits into one request.
	big := make([][4]int, 50)
	for i := range big {
		big[i] = [4]int{100 + i, i % 25, 0, 1}
	}
	if c := postJSON(t, h, loc+"/mark", map[string]any{"token": joined.Token, "round": 1, "ops": big}, nil); c != http.StatusOK {
		t.Fatalf("big mark: %d", c)
	}
	var voted struct{ Snapshot Snapshot }
	if c := postJSON(t, h, loc+"/vote", map[string]any{"token": joined.Token, "round": 1, "giveUp": true}, &voted); c != http.StatusOK || voted.Snapshot.Phase != PhaseResults {
		t.Fatalf("vote: %d %+v", c, voted.Snapshot)
	}
}
