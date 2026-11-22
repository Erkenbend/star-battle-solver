//go:build unix

package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"starbattle"
)

// GzipStressEnv runs TestGzipStress. It runs nowhere by default, only on
// request (it takes a few minutes and its numbers depend on the machine):
//
//	STARBATTLE_GZIP_STRESS=1 go test ./internal/web -run TestGzipStress -v -timeout 30m
//
// On the Raspberry Pi, build the test binary here (it carries the hard
// puzzles and the assets) and run it there, ideally with the service
// stopped:
//
//	GOOS=linux GOARCH=arm64 go test -c -o web.test ./internal/web
//	scp web.test pi@raspberrypi.local:
//	ssh pi@raspberrypi.local 'STARBATTLE_GZIP_STRESS=1 ./web.test -test.run TestGzipStress -test.v -test.timeout 30m'
//
// STARBATTLE_GZIP_STRESS_SECONDS sets how long each load phase runs
// (default 5). -test.cpu 4 limits a desktop to the Pi's four cores.
const GzipStressEnv = "STARBATTLE_GZIP_STRESS"

// TestGzipStress checks the hypotheses of suggestion 7 in
// architecture/refactorings.md and measures what decides the level:
//
//  1. Size: a 15×15 hard puzzle page is about 208 KB and gzip makes it
//     about 56 KB (fails if gzipped is above 35 % of the original).
//  2. CPU per page: what gzip adds to serving a page, in process, at the
//     levels BestSpeed, Default and BestCompression.
//  3. Load: many players loading at once (a party's next round): requests
//     per second, latency and CPU per request with and without gzip, for a
//     page that is only rendered (library, solved once) and one that is
//     generated and solved on every request.
//  4. Slow connection: time until a page is in the browser (including
//     decoding) at a few bandwidths, with and without gzip.
//
// It only logs its numbers, apart from the size check of 1. Record them in
// the decision that comes with the change.
func TestGzipStress(t *testing.T) {
	if os.Getenv(GzipStressEnv) == "" || testing.Short() {
		t.Skip("set " + GzipStressEnv + "=1 to run")
	}
	phase := 5 * time.Second
	if s := os.Getenv(GzipStressEnv + "_SECONDS"); s != "" {
		v, err := strconv.Atoi(s)
		must(t, err)
		phase = time.Duration(v) * time.Second
	}
	t.Logf("GOOS %s, GOARCH %s, GOMAXPROCS %d, %d s per load phase", runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), int(phase.Seconds()))

	lib := testLibrary(t)
	syms, err := LoadSymbols(starbattle.Symbols, "symbols")
	must(t, err)
	h := Handler(testSolve, nil, nil, []*Library{lib}, syms)

	// A largest hard puzzle: the largest page, with the longest trace.
	var big *libEntry
	for _, e := range lib.entries {
		if big == nil || e.p.N > big.p.N {
			big = e
		}
	}
	if rec := get(h, "/bibliothek/weg?id="+big.id); rec.Code != http.StatusOK { // waits for the solver
		t.Fatalf("trace of %s: %d", big.name(), rec.Code)
	}
	bigPage := "/bibliothek?id=" + big.id

	pages := []struct{ name, path string }{
		{fmt.Sprintf("hard %s page", big.name()), bigPage},
		{fmt.Sprintf("hard %s trace (JSON)", big.name()), "/bibliothek/weg?id=" + big.id},
		{"generated 8×8 page", "/?n=8&seed=42"},
		{"generated 15×15 page", "/?n=15&seed=42"},
		{"tutorial", "/anleitung"},
		{"library overview", "/bibliothek"},
		{"settings", "/einstellungen"},
		{"symbols (JSON)", "/symbole.json"},
		{"error page", "/?n=99"},
	}

	t.Run("1 size", func(t *testing.T) {
		t.Logf("%-26s %9s %9s %9s %9s %9s", "", "raw", "sent", "speed", "default", "best")
		for _, pg := range pages {
			raw := get(h, pg.path).Body.Bytes()
			sent := serveGzip(h, pg.path).Body.Len()
			t.Logf("%-26s %9s %9s %9s %9s %9s", pg.name, kb(len(raw)), kb(sent),
				kb(gzipLen(raw, gzip.BestSpeed)), kb(gzipLen(raw, gzip.DefaultCompression)), kb(gzipLen(raw, gzip.BestCompression)))
			if pg.path == bigPage && sent*100 > len(raw)*35 {
				t.Errorf("hypothesis: %s gzipped %s of %s, more than 35 %%", pg.name, kb(sent), kb(len(raw)))
			}
		}
	})

	t.Run("2 cpu per page", func(t *testing.T) {
		t.Logf("%-26s %10s %10s | %10s %10s %10s", "", "serve", "+gzip", "speed", "default", "best")
		for _, pg := range pages {
			if pg.path == "/?n=15&seed=42" {
				continue // generating dominates; see 3
			}
			plain := median(func() { get(h, pg.path) })
			zipped := median(func() { serveGzip(h, pg.path) })
			raw := get(h, pg.path).Body.Bytes()
			lv := func(level int) time.Duration {
				zw, _ := gzip.NewWriterLevel(io.Discard, level)
				return median(func() { zw.Reset(io.Discard); zw.Write(raw); zw.Close() })
			}
			t.Logf("%-26s %10s %10s | %10s %10s %10s", pg.name, us(plain), us(zipped-plain), us(lv(gzip.BestSpeed)), us(lv(gzip.DefaultCompression)), us(lv(gzip.BestCompression)))
		}
	})

	t.Run("3 load", func(t *testing.T) {
		srv := httptest.NewServer(h)
		defer srv.Close()
		var seed atomic.Uint64
		seed.Store(1000)
		loads := []struct {
			name string
			path func() string
		}{
			{"hard " + big.name() + " page", func() string { return bigPage }},
			{"generated 8×8, new seed", func() string { return fmt.Sprintf("/?n=8&seed=%d", seed.Add(1)) }},
		}
		t.Logf("%-26s %4s %5s %8s %8s %8s %8s %10s %9s", "", "conc", "gzip", "req/s", "p50", "p95", "max", "cpu/req", "KB/req")
		for _, ld := range loads {
			for _, conc := range []int{1, 16, 64} {
				for _, on := range []bool{false, true} {
					r := loadPhase(t, srv.URL, ld.path, conc, on, phase)
					t.Logf("%-26s %4d %5v %8.1f %8s %8s %8s %10s %9.1f", ld.name, conc, on, r.perSec, ms(r.p50), ms(r.p95), ms(r.max), us(r.cpuPerReq), r.kbPerReq)
				}
			}
		}
	})

	t.Run("4 slow connection", func(t *testing.T) {
		// Bytes per second from server to browser: about DevTools' "Slow
		// 3G" and "Fast 3G", a weak Wi-Fi and a good one.
		t.Logf("%-26s %10s %10s %10s %8s", "", "KB/s", "plain", "gzip", "saved")
		for _, rate := range []int{50_000, 200_000, 1_000_000, 5_000_000} {
			srv := httptest.NewUnstartedServer(h)
			srv.Listener = slowListener{srv.Listener, rate}
			srv.Start()
			for _, pg := range pages[:3] {
				plain := slowFetch(t, srv.URL+pg.path, false)
				zipped := slowFetch(t, srv.URL+pg.path, true)
				t.Logf("%-26s %10d %10s %10s %7.0f%%", pg.name, rate/1000, ms(plain), ms(zipped), 100*(1-float64(zipped)/float64(plain)))
			}
			srv.Close()
		}
	})
}

// serveGzip requests path from h in process, as a browser that accepts gzip.
func serveGzip(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	h.ServeHTTP(rec, req)
	return rec
}

func gzipLen(b []byte, level int) int {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, level)
	zw.Write(b)
	zw.Close()
	return buf.Len()
}

// median runs f for about 300 ms (at least 5 times) and returns the median
// time of one run.
func median(f func()) time.Duration {
	f() // warm up
	var ds []time.Duration
	for start := time.Now(); len(ds) < 5 || time.Since(start) < 300*time.Millisecond; {
		t := time.Now()
		f()
		ds = append(ds, time.Since(t))
	}
	slices.Sort(ds)
	return ds[len(ds)/2]
}

type loadResult struct {
	perSec, kbPerReq float64
	p50, p95, max    time.Duration
	cpuPerReq        time.Duration
}

// loadPhase lets conc players request path() from base for d, with or without
// gzip. The players read the answers without decoding them, so the CPU
// time of the process is the server's plus little else.
func loadPhase(t *testing.T, base string, path func() string, conc int, on bool, d time.Duration) loadResult {
	t.Helper()
	tr := &http.Transport{DisableCompression: true, MaxIdleConnsPerHost: conc}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr}
	runtime.GC()
	cpu0 := cpuTime()
	start := time.Now()
	var (
		mu     sync.Mutex
		lat    []time.Duration
		total  int64
		failed error
		wg     sync.WaitGroup
	)
	fail := func(err error) {
		mu.Lock()
		if failed == nil {
			failed = err
		}
		mu.Unlock()
	}
	for range conc {
		wg.Go(func() {
			for time.Since(start) < d {
				req, _ := http.NewRequest("GET", base+path(), nil)
				if on {
					req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
				}
				t0 := time.Now()
				res, err := client.Do(req)
				if err != nil {
					fail(err)
					return
				}
				n, err := io.Copy(io.Discard, res.Body)
				res.Body.Close()
				if err != nil || res.StatusCode != http.StatusOK || (res.Header.Get("Content-Encoding") == "gzip") != on {
					fail(fmt.Errorf("status %d, encoding %q, %v", res.StatusCode, res.Header.Get("Content-Encoding"), err))
					return
				}
				l := time.Since(t0)
				mu.Lock()
				lat = append(lat, l)
				total += n
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	wall := time.Since(start)
	cpu := cpuTime() - cpu0
	if failed != nil {
		t.Fatal(failed)
	}
	slices.Sort(lat)
	n := len(lat)
	return loadResult{
		perSec:    float64(n) / wall.Seconds(),
		kbPerReq:  float64(total) / float64(n) / 1000,
		p50:       lat[n/2],
		p95:       lat[n*95/100],
		max:       lat[n-1],
		cpuPerReq: cpu / time.Duration(n),
	}
}

// cpuTime is the user and system time of the process so far.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// slowFetch loads url once on a new connection, as a browser that accepts
// gzip or not, and returns the median time of three loads until the page
// is decoded.
func slowFetch(t *testing.T, url string, on bool) time.Duration {
	t.Helper()
	var ds []time.Duration
	for range 3 {
		tr := &http.Transport{DisableCompression: true, DisableKeepAlives: true}
		req, _ := http.NewRequest("GET", url, nil)
		if on {
			req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
		}
		t0 := time.Now()
		res, err := (&http.Client{Transport: tr}).Do(req)
		must(t, err)
		var body io.Reader = res.Body
		if res.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(res.Body)
			must(t, err)
			body = zr
		}
		_, err = io.Copy(io.Discard, body)
		must(t, err)
		res.Body.Close()
		ds = append(ds, time.Since(t0))
		tr.CloseIdleConnections()
	}
	slices.Sort(ds)
	return ds[1]
}

// slowListener hands out connections that send at most rate bytes per
// second (the server's answers; requests are small).
type slowListener struct {
	net.Listener
	rate int
}

func (l slowListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &slowConn{Conn: c, rate: l.rate}, nil
}

type slowConn struct {
	net.Conn
	rate  int
	start time.Time // when sending began
	sent  int       // bytes since start
}

// Write sends b in packets of 1460 bytes, each no earlier than the rate
// allows since the connection's first write.
func (c *slowConn) Write(b []byte) (int, error) {
	if c.start.IsZero() {
		c.start = time.Now()
	}
	n := 0
	for len(b) > 0 {
		k := min(len(b), 1460)
		c.sent += k
		time.Sleep(time.Until(c.start.Add(time.Duration(c.sent) * time.Second / time.Duration(c.rate))))
		m, err := c.Conn.Write(b[:k])
		n += m
		if err != nil {
			return n, err
		}
		b = b[k:]
	}
	return n, nil
}

func kb(n int) string           { return fmt.Sprintf("%.1f KB", float64(n)/1000) }
func us(d time.Duration) string { return fmt.Sprintf("%.0f µs", float64(d)/float64(time.Microsecond)) }
func ms(d time.Duration) string { return fmt.Sprintf("%.0f ms", float64(d)/float64(time.Millisecond)) }
