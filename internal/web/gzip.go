package web

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzipLevel is the level pages and JSON are compressed with: the fastest,
// since the server may run on a Raspberry Pi. Decision 34 has what the
// other levels would save and cost (BenchmarkGzipLevels).
const gzipLevel = gzip.BestSpeed

// gzipTypes are the content types worth compressing: the pages (about
// 200 KB of mostly script and style) and the JSON answers (traces of
// library puzzles, party snapshots). Not the event stream, which must
// reach the player per event, and not the PNG icons, which are compressed
// already.
var gzipTypes = map[string]bool{
	"text/html":        true,
	"application/json": true,
}

var gzipWriters = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(io.Discard, gzipLevel)
	return w
}}

// Compress gzips the answers of h whose content type is in gzipTypes, when
// the request accepts gzip. It decides at the first write of a handler (or
// its WriteHeader), so handlers stay unaware of it. serve wraps Handler in
// it unless started with -gzip=false, e.g. on a weak CPU in a fast network
// (decision 34).
func Compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponse{ResponseWriter: w}
		defer gw.close()
		h.ServeHTTP(gw, r)
	})
}

// acceptsGzip reports whether an Accept-Encoding header allows gzip: it
// names gzip with a quality above 0, or else * with one.
func acceptsGzip(header string) bool {
	star := false
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "gzip" && name != "*" {
			continue
		}
		ok := true
		if k, v, _ := strings.Cut(strings.TrimSpace(params), "="); strings.TrimSpace(k) == "q" {
			q, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			ok = err == nil && q > 0
		}
		if name == "gzip" {
			return ok
		}
		star = ok
	}
	return star
}

// gzipResponse is the ResponseWriter Compress hands to the handler.
type gzipResponse struct {
	http.ResponseWriter
	decided bool         // whether to compress is settled (at the first WriteHeader or Write)
	on      bool         // compress this answer
	gz      *gzip.Writer // from gzipWriters, once there is a body
}

func (g *gzipResponse) WriteHeader(status int) {
	if !g.decided && status >= 200 { // not on informational answers such as 103
		g.decided = true
		h := g.Header()
		ct, _, _ := mime.ParseMediaType(h.Get("Content-Type"))
		g.on = gzipTypes[ct] && h.Get("Content-Encoding") == "" &&
			status != http.StatusNoContent && status != http.StatusNotModified
		if g.on {
			h.Set("Content-Encoding", "gzip")
			h.Del("Content-Length")
		}
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponse) Write(b []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if !g.on {
		return g.ResponseWriter.Write(b)
	}
	if g.gz == nil {
		if len(b) == 0 {
			return 0, nil
		}
		g.gz = gzipWriters.Get().(*gzip.Writer)
		g.gz.Reset(g.ResponseWriter)
	}
	return g.gz.Write(b)
}

// FlushError lets http.ResponseController flush through the compressor.
// No compressed answer is flushed today; the event stream is not compressed.
func (g *gzipResponse) FlushError() error {
	if g.gz != nil {
		if err := g.gz.Flush(); err != nil {
			return err
		}
	}
	return http.NewResponseController(g.ResponseWriter).Flush()
}

// Unwrap gives http.ResponseController the underlying writer, e.g. for the
// event stream's SetWriteDeadline.
func (g *gzipResponse) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipResponse) close() {
	if g.gz == nil {
		return
	}
	g.gz.Close()
	g.gz.Reset(io.Discard)
	gzipWriters.Put(g.gz)
	g.gz = nil
}
