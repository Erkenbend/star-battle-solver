package web

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/icon"
	"starbattle/internal/party"
)

// rawGet requests path from srv with the given Accept-Encoding (none if
// empty) and returns the answer with its body as sent, not decoded.
func rawGet(t *testing.T, srv *httptest.Server, path, enc string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest("GET", srv.URL+path, nil)
	must(t, err)
	if enc != "" {
		req.Header.Set("Accept-Encoding", enc)
	}
	tr := &http.Transport{DisableCompression: true}
	defer tr.CloseIdleConnections()
	res, err := (&http.Client{Transport: tr}).Do(req)
	must(t, err)
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	must(t, err)
	return res, b
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	must(t, err)
	out, err := io.ReadAll(zr)
	must(t, err)
	return out
}

// TestGzip: pages and JSON go out gzipped to a browser that accepts it and
// decode to what a browser without gzip gets; the event stream, the icons
// and the manifest go out as they are.
func TestGzip(t *testing.T) {
	lib := testLibrary(t)
	pm, _ := newTestParties(t)
	srv := httptest.NewServer(Compress(Handler(testSolve, nil, pm, []*Library{lib}, nil)))
	defer srv.Close()
	e := lib.entries[0]

	// The trace first: /bibliothek/weg waits for the solver, so the page
	// after it is the same on every request.
	for _, path := range []string{"/bibliothek/weg?id=" + e.id, "/bibliothek?id=" + e.id, "/?n=6&seed=3", "/bibliothek"} {
		plain, want := rawGet(t, srv, path, "")
		if plain.StatusCode != http.StatusOK || plain.Header.Get("Content-Encoding") != "" {
			t.Fatalf("%s without gzip: %d, encoding %q", path, plain.StatusCode, plain.Header.Get("Content-Encoding"))
		}
		res, b := rawGet(t, srv, path, "gzip, deflate, br")
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "gzip" || !strings.Contains(res.Header.Get("Vary"), "Accept-Encoding") {
			t.Fatalf("%s with gzip: %d, encoding %q, vary %q", path, res.StatusCode, res.Header.Get("Content-Encoding"), res.Header.Get("Vary"))
		}
		if res.Header.Get("Content-Type") != plain.Header.Get("Content-Type") || res.Header.Get("Cache-Control") != plain.Header.Get("Cache-Control") {
			t.Errorf("%s: headers %v, without gzip %v", path, res.Header, plain.Header)
		}
		if got := gunzip(t, b); !bytes.Equal(got, want) {
			t.Errorf("%s: decodes to %d bytes, without gzip %d", path, len(got), len(want))
		}
		// About a third (decision 34); more means a lost level or a page
		// that no longer compresses well.
		if len(want) > 10000 && len(b)*100 > len(want)*40 {
			t.Errorf("%s: %d bytes gzipped, %d without: more than 40 %%", path, len(b), len(want))
		}
		if res, _ := rawGet(t, srv, path, "gzip;q=0, identity"); res.Header.Get("Content-Encoding") != "" {
			t.Errorf("%s: gzip with q=0 sent %q", path, res.Header.Get("Content-Encoding"))
		}
	}

	// Error pages are HTML too; plain-text errors stay as they are.
	if res, b := rawGet(t, srv, "/?n=99", "gzip"); res.StatusCode != http.StatusBadRequest || res.Header.Get("Content-Encoding") != "gzip" || !bytes.Contains(gunzip(t, b), []byte("zwischen")) {
		t.Errorf("error page: %d %q", res.StatusCode, res.Header.Get("Content-Encoding"))
	}
	if res, _ := rawGet(t, srv, "/bibliothek/weg?id=nix", "gzip"); res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Encoding") != "" {
		t.Errorf("plain-text error: %d %q", res.StatusCode, res.Header.Get("Content-Encoding"))
	}

	for _, path := range []string{"/" + icon.All[0].File, "/manifest.webmanifest"} {
		if res, _ := rawGet(t, srv, path, "gzip"); res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "" {
			t.Errorf("%s: %d, encoding %q", path, res.StatusCode, res.Header.Get("Content-Encoding"))
		}
	}

	// The event stream is not compressed and its first event arrives at once.
	code, err := pm.Create(party.Options{Sizes: []int{5}})
	must(t, err)
	tok := mustJoin(t, pm, code, "Anna")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/party/"+code+"/events?token="+tok, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	tr := &http.Transport{DisableCompression: true}
	defer tr.CloseIdleConnections()
	res, err := (&http.Client{Transport: tr}).Do(req)
	must(t, err)
	defer res.Body.Close()
	if res.Header.Get("Content-Encoding") != "" || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("event stream: encoding %q, type %q", res.Header.Get("Content-Encoding"), res.Header.Get("Content-Type"))
	}
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() && !strings.HasPrefix(sc.Text(), "data: ") {
	}
	if !strings.Contains(sc.Text(), code) {
		t.Fatalf("first event: %q %v", sc.Text(), sc.Err())
	}
}

func TestAcceptsGzip(t *testing.T) {
	for _, c := range []struct {
		header string
		want   bool
	}{
		{"", false},
		{"gzip", true},
		{"gzip, deflate, br, zstd", true},
		{"deflate, GZIP;q=0.5", true},
		{"*", true},
		{"identity", false},
		{"gzip;q=0", false},
		{"gzip; q=0.000, deflate", false},
		{"br;q=1.0, gzip;q=0.8, *;q=0.1", true},
		{"x-gzip", false},
		{"*, gzip;q=0", false},
		{"gzip;q=0, *", false},
		{"*;q=0, gzip", true},
	} {
		if got := acceptsGzip(c.header); got != c.want {
			t.Errorf("acceptsGzip(%q) = %v", c.header, got)
		}
	}
}

// BenchmarkGzipLevels compresses the largest page, a generated one and the
// tutorial at three levels, for decision 34: time per page and the size
// sent (B/page).
//
//	go test ./internal/web -run '^$' -bench GzipLevels
func BenchmarkGzipLevels(b *testing.B) {
	lib, err := LoadLibrary(starbattle.Hard, "testdata/hard", testSolve)
	if err != nil {
		b.Fatal(err)
	}
	h := Handler(testSolve, nil, nil, []*Library{lib}, nil)
	big := lib.entries[0]
	for _, e := range lib.entries {
		if e.p.N > big.p.N {
			big = e
		}
	}
	get(h, "/bibliothek/weg?id="+big.id) // waits for the solve, so the page has the trace
	pages := []struct{ name, path string }{
		{"hard" + big.name(), "/bibliothek?id=" + big.id},
		{"generated8", "/?n=8&seed=42"},
		{"tutorial", "/anleitung"},
	}
	for _, pg := range pages {
		page := get(h, pg.path).Body.Bytes()
		b.Run(fmt.Sprintf("%s/raw", pg.name), func(b *testing.B) {
			for b.Loop() {
				get(h, pg.path)
			}
			b.ReportMetric(float64(len(page)), "B/page")
		})
		for _, level := range []int{gzip.BestSpeed, gzip.DefaultCompression, gzip.BestCompression} {
			b.Run(fmt.Sprintf("%s/level%d", pg.name, level), func(b *testing.B) {
				var buf bytes.Buffer
				zw, _ := gzip.NewWriterLevel(&buf, level)
				for b.Loop() {
					buf.Reset()
					zw.Reset(&buf)
					zw.Write(page)
					zw.Close()
				}
				b.ReportMetric(float64(buf.Len()), "B/page")
			})
		}
	}
}
