// Package stress drives a running `starbattle serve` over HTTP through load
// scenarios and reports the latency the visitors see and the CPU time and
// memory of the server (decision 33). It is a developer tool, behind
// `starbattle stress`: use it to compare builds after a change to how the
// server schedules its work (the solve budget, the pool, the cache).
package stress

import (
	"cmp"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"starbattle/internal/collection"
)

// Config says which server to drive and with what.
type Config struct {
	URL string // the server, e.g. http://localhost:6391
	// PID is the server's process, for its CPU time and memory (read from
	// /proc, so Linux only); 0 measures nothing.
	PID int
	// Hard and Motifs are the library's puzzles: hard ones are pasted into
	// /play and opened by the crawler, motifs opened during the crawl.
	Hard, Motifs []collection.Entry
	// HeavyN is the size of the hard puzzles pasted into /play (and
	// HeavyN+1); 0 means 14, which takes about a second to solve on a
	// desktop core.
	HeavyN int
	Out    io.Writer
}

// Scenario is a load pattern.
type Scenario struct {
	Name string
	Doc  string
	run  func(*runner)
}

// Scenarios are all scenarios, in the order `starbattle stress` runs them.
var Scenarios = []Scenario{
	{"tutorial", "200 visits of /anleitung, one after another", (*runner).tutorial},
	{"reload", "a hard pasted puzzle reloaded 10 times, then 8 players open another one at once", (*runner).reload},
	{"abandon", "12 visitors paste hard puzzles and leave after 300 ms", (*runner).abandon},
	{"flood", "12 visitors paste hard puzzles at once and stay", (*runner).flood},
	{"crawl", "a crawler opens every hard puzzle of the library while a player asks for new puzzles; takes about a minute on 4 cores", (*runner).crawl},
	{"pages", "16 players load a page at once, 50 times, without and with gzip: a solved hard library page, new 8×8 puzzles, the tutorial", (*runner).pages},
}

// Find returns the scenario called name.
func Find(name string) (Scenario, bool) {
	i := slices.IndexFunc(Scenarios, func(s Scenario) bool { return s.Name == name })
	if i < 0 {
		return Scenario{}, false
	}
	return Scenarios[i], true
}

// Run runs s against c's server. The error counts the requests that failed;
// they are printed as they happen.
func Run(c Config, s Scenario) error {
	if c.HeavyN == 0 {
		c.HeavyN = 14
	}
	r := &runner{Config: c}
	fmt.Fprintf(c.Out, "== %s: %s\n", s.Name, s.Doc)
	s.run(r)
	if r.failed > 0 {
		return fmt.Errorf("%s: %d requests failed", s.Name, r.failed)
	}
	return nil
}

type runner struct {
	Config
	mu     sync.Mutex
	failed int
}

// cpu is the server's user and system time so far.
func (r *runner) cpu() time.Duration {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", r.PID))
	if r.PID == 0 || err != nil {
		return 0
	}
	s := string(b)
	f := strings.Fields(s[strings.LastIndex(s, ")")+2:]) // the name may contain spaces
	user, _ := strconv.Atoi(f[11])
	sys, _ := strconv.Atoi(f[12])
	return time.Duration(user+sys) * 10 * time.Millisecond // clock ticks, 100 per second on Linux
}

// status is a line of the server's /proc/PID/status, e.g. VmHWM.
func (r *runner) status(key string) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", r.PID))
	if r.PID == 0 || err != nil {
		return "n/a"
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "n/a"
}

// report prints the time since t0 and the server's CPU since c0 and memory.
func (r *runner) report(label string, c0 time.Duration, t0 time.Time) {
	fmt.Fprintf(r.Out, "  %-30s wall %6.2fs  server CPU %7.2fs  RSS %s  peak %s\n",
		label, time.Since(t0).Seconds(), (r.cpu() - c0).Seconds(), r.status("VmRSS"), r.status("VmHWM"))
}

// get requests path as a browser does: http.DefaultClient asks for gzip and
// decodes it unseen, so every scenario measures what serve sends by
// default. pages chooses the encoding itself (fetch).
func (r *runner) get(ctx context.Context, path string) (int, time.Duration, error) {
	t := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", r.URL+path, nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, time.Since(t), err
	}
	_, err = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode, time.Since(t), err
}

// must gets path and counts anything but 200 as a failure.
func (r *runner) must(path string) time.Duration {
	code, d, err := r.get(context.Background(), path)
	if err != nil || code != http.StatusOK {
		r.mu.Lock()
		r.failed++
		r.mu.Unlock()
		fmt.Fprintf(r.Out, "  FAILED %.60s: status %d %v\n", path, code, err)
	}
	return d
}

// latencies collects durations from several goroutines.
type latencies struct {
	mu sync.Mutex
	ds []time.Duration
}

func (l *latencies) add(d time.Duration) {
	l.mu.Lock()
	l.ds = append(l.ds, d)
	l.mu.Unlock()
}

func (l *latencies) String() string {
	if len(l.ds) == 0 {
		return "none"
	}
	s := slices.Clone(l.ds)
	slices.Sort(s)
	return fmt.Sprintf("n=%d median %.2fs slowest %.2fs", len(s), s[len(s)/2].Seconds(), s[len(s)-1].Seconds())
}

// ms is like String, in milliseconds and with the 95th percentile, for
// requests that take less than a second.
func (l *latencies) ms() string {
	s := slices.Clone(l.ds)
	slices.Sort(s)
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return fmt.Sprintf("n=%d median %.1f ms, p95 %.1f ms, slowest %.1f ms", len(s), ms(s[len(s)/2]), ms(s[len(s)*95/100]), ms(s[len(s)-1]))
}

// play is the /play link of e's puzzle.
func play(e collection.Entry) string {
	rows := strings.ReplaceAll(strings.TrimSpace(e.Puzzle.String()), "\n", "/")
	return "/play?" + url.Values{"p": {rows}}.Encode()
}

// heavy returns up to k hard puzzles of size HeavyN, then HeavyN+1.
func (r *runner) heavy(k int) []collection.Entry {
	var out []collection.Entry
	for _, n := range []int{r.HeavyN, r.HeavyN + 1} {
		for _, e := range r.Hard {
			if e.Puzzle.N == n && len(out) < k {
				out = append(out, e)
			}
		}
	}
	return out
}

func (r *runner) tutorial() {
	c0, t0 := r.cpu(), time.Now()
	var l latencies
	for range 200 {
		l.add(r.must("/anleitung"))
	}
	r.report("200 × /anleitung", c0, t0)
	fmt.Fprintf(r.Out, "  latency: %v\n", &l)
}

func (r *runner) reload() {
	hs := r.heavy(2)
	if len(hs) < 2 {
		fmt.Fprintf(r.Out, "  needs two hard puzzles of size %d or %d\n", r.HeavyN, r.HeavyN+1)
		r.failed++
		return
	}
	c0, t0 := r.cpu(), time.Now()
	var seq latencies
	for range 10 {
		seq.add(r.must(play(hs[0])))
	}
	r.report("10 reloads", c0, t0)
	fmt.Fprintf(r.Out, "  latency: %v (first %.2fs)\n", &seq, seq.ds[0].Seconds())

	c0, t0 = r.cpu(), time.Now()
	var par latencies
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { par.add(r.must(play(hs[1]))) })
	}
	wg.Wait()
	r.report("8 players open a link at once", c0, t0)
	fmt.Fprintf(r.Out, "  latency: %v\n", &par)

	c0, t0 = r.cpu(), time.Now()
	d := r.must(play(hs[1]))
	r.report("that link once more", c0, t0)
	fmt.Fprintf(r.Out, "  latency: %.2fs\n", d.Seconds())
}

// abandon: after the visitors left, the server's CPU shows whether their
// solves go on.
func (r *runner) abandon() {
	c0, t0 := r.cpu(), time.Now()
	var wg sync.WaitGroup
	for _, e := range r.heavy(12) {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			r.get(ctx, play(e)) // fails by design
		})
	}
	wg.Wait()
	r.report("12 visitors gone", c0, t0)
	c1 := r.cpu()
	fmt.Fprintf(r.Out, "  a new 8×8 right after: %.2fs\n", r.must("/?n=8&seed=777777").Seconds())
	for _, s := range []time.Duration{1, 2, 4, 8} {
		time.Sleep(time.Until(t0.Add(s * time.Second)))
		fmt.Fprintf(r.Out, "  +%ds: server CPU since they left %.2fs\n", s, (r.cpu() - c1).Seconds())
	}
}

func (r *runner) flood() {
	c0, t0 := r.cpu(), time.Now()
	var l latencies
	var wg sync.WaitGroup
	for _, e := range r.heavy(12) {
		wg.Go(func() { l.add(r.must(play(e))) })
	}
	wg.Wait()
	r.report("12 pastes at once", c0, t0)
	fmt.Fprintf(r.Out, "  latency: %v\n", &l)
}

// crawl: the hard puzzles' pages come before their solves; /bibliothek/weg
// waits for a solve, so asking it for every puzzle waits for all of them.
func (r *runner) crawl() {
	c0, t0 := r.cpu(), time.Now()
	stop := make(chan struct{})
	var player latencies
	var wg sync.WaitGroup
	wg.Go(func() {
		for seed := 900001; ; seed++ {
			select {
			case <-stop:
				return
			case <-time.After(300 * time.Millisecond):
			}
			player.add(r.must(fmt.Sprintf("/?n=10&seed=%d", seed))) // not in the pool
		}
	})
	var pages latencies
	for _, e := range r.Hard {
		pages.add(r.must("/bibliothek?id=" + e.ID))
	}
	r.report("crawler done", c0, t0)
	fmt.Fprintf(r.Out, "  crawler's pages: %v\n", &pages)
	var motifs latencies
	for _, e := range r.Motifs[:min(3, len(r.Motifs))] {
		motifs.add(r.must("/bibliothek?id=" + e.ID)) // waits for its solve
	}
	fmt.Fprintf(r.Out, "  motif pages during the crawl: %v\n", &motifs)
	sem := make(chan struct{}, 32)
	var weg sync.WaitGroup
	for _, e := range r.Hard {
		weg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			r.must("/bibliothek/weg?id=" + e.ID)
		})
	}
	weg.Wait()
	close(stop)
	wg.Wait()
	r.report("every library solve done", c0, t0)
	fmt.Fprintf(r.Out, "  new 10×10 during the crawl: %v\n", &player)
}

// raw sends requests as they are: no Accept-Encoding of its own, answers
// not decoded, so fetch sees what goes over the wire.
var raw = &http.Client{Transport: &http.Transport{DisableCompression: true, MaxIdleConnsPerHost: 64}}

// fetch gets path asking for gzip or not, decodes the answer as a browser
// would and returns the time until it is decoded, the bytes sent and their
// encoding. Anything but 200 counts as a failure.
func (r *runner) fetch(path string, gz bool) (time.Duration, int64, string) {
	t := time.Now()
	req, err := http.NewRequest("GET", r.URL+path, nil)
	if err != nil {
		panic(err)
	}
	if gz {
		req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd") // as Chromium
	}
	resp, err := raw.Do(req)
	var sent counter
	enc := ""
	if err == nil {
		enc = resp.Header.Get("Content-Encoding")
		var body io.Reader = io.TeeReader(resp.Body, &sent)
		if enc == "gzip" {
			var zr *gzip.Reader
			if zr, err = gzip.NewReader(body); err == nil {
				body = zr
			}
		}
		if err == nil {
			_, err = io.Copy(io.Discard, body)
		}
		resp.Body.Close()
		if err == nil && resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("status %d", resp.StatusCode)
		}
	}
	if err != nil {
		r.mu.Lock()
		r.failed++
		r.mu.Unlock()
		fmt.Fprintf(r.Out, "  FAILED %.60s: %v\n", path, err)
	}
	return time.Since(t), sent.n, cmp.Or(enc, "none")
}

type counter struct{ n int64 }

func (c *counter) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return len(b), nil
}

// pages: what gzip costs the server and saves on the wire, for pages whose
// solve is done (a hard library page, the tutorial) and for new puzzles,
// generated and solved per request. 16 players load at once, like a party
// starting its next round, 50 times; once without gzip, once with.
func (r *runner) pages() {
	big := r.Hard[0]
	for _, e := range r.Hard {
		if e.Puzzle.N > big.Puzzle.N {
			big = e
		}
	}
	r.must("/bibliothek/weg?id=" + big.ID) // waits for the solve: then the page has the trace
	seed := 800000
	kinds := []struct {
		name string
		path func() string
	}{
		{fmt.Sprintf("hard %d×%d page", big.Puzzle.N, big.Puzzle.N), func() string { return "/bibliothek?id=" + big.ID }},
		{"new 8×8", func() string { seed++; return fmt.Sprintf("/?n=8&seed=%d", seed) }}, // not in the cache
		{"tutorial", func() string { return "/anleitung" }},
	}
	const players, rounds = 16, 50
	for _, k := range kinds {
		for _, gz := range []bool{false, true} {
			c0, t0 := r.cpu(), time.Now()
			var l latencies
			var sent int64
			enc := ""
			var mu sync.Mutex
			for range rounds {
				var wg sync.WaitGroup
				for range players {
					path := k.path() // in this goroutine: seed is not shared
					wg.Go(func() {
						d, n, e := r.fetch(path, gz)
						l.add(d)
						mu.Lock()
						sent += n
						enc = e
						mu.Unlock()
					})
				}
				wg.Wait()
			}
			cpu := r.cpu() - c0
			r.report(fmt.Sprintf("%s, gzip %v", k.name, gz), c0, t0)
			n := players * rounds
			fmt.Fprintf(r.Out, "  latency: %s; per page: server CPU %.2f ms, %.1f KB sent (%s)\n",
				l.ms(), float64(cpu)/float64(n)/float64(time.Millisecond), float64(sent)/float64(n)/1000, enc)
		}
	}
}
