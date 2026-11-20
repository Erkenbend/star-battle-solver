package web

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	starbattle "starbattle"
)

const manifestLink = `<link rel="manifest" href="/manifest.webmanifest" crossorigin="use-credentials">`

// Every page the server sends links the manifest (with credentials, for a
// site behind basic auth); a page saved with `starbattle html` does not, it
// has no server.
func TestInstallLinks(t *testing.T) {
	pm, _ := newTestParties(t)
	syms, err := LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		t.Fatal(err)
	}
	lib := testLibrary(t)
	h := Handler(testSolve, nil, pm, []*Library{lib}, syms)
	pages := []string{
		"/?n=5&seed=1", "/play?p=AABBB/ACCBB/CCCDB/EEDDB/EEEEE", "/anleitung",
		"/bibliothek", "/bibliothek?id=" + lib.entries[0].id, "/einstellungen", "/party",
		"/?n=99", // error page
	}
	for _, p := range pages {
		rec := get(h, p)
		if !strings.Contains(rec.Body.String(), manifestLink) {
			t.Errorf("%s (%d): no manifest link", p, rec.Code)
		}
	}

	var buf bytes.Buffer
	if err := Write(&buf, build(load(t, "../../testdata/hard/n05_s029.txt"))); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "manifest") {
		t.Error("the page of `starbattle html` links a manifest")
	}
}

// The manifest makes the site installable as an app of its own window, and
// every icon it names is a PNG of the size it states.
func TestManifest(t *testing.T) {
	h := Handler(testSolve, nil, nil, nil, nil)
	rec := get(h, "/manifest.webmanifest")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var m Manifest
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name == "" || m.StartURL != "/" || m.Scope != "/" || m.Display != "standalone" {
		t.Fatalf("manifest: %+v", m)
	}
	var purposes []string
	for _, ic := range m.Icons {
		purposes = append(purposes, ic.Purpose)
	}
	if !slices.Contains(purposes, "any") || !slices.Contains(purposes, "maskable") {
		t.Errorf("icon purposes %v", purposes)
	}

	// The icons of the manifest and those of the pages' head.
	urls := map[string]string{"/icon-32.png": "32x32", "/apple-touch-icon.png": "180x180"}
	for _, ic := range m.Icons {
		urls[ic.Src] = ic.Sizes
	}
	for u, size := range urls {
		rec := get(h, u)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("%s: %d %q", u, rec.Code, rec.Header().Get("Content-Type"))
			continue
		}
		img, err := png.Decode(rec.Body)
		if err != nil {
			t.Errorf("%s: %v", u, err)
			continue
		}
		if b := img.Bounds(); sizes(b.Dx()) != size || b.Dx() != b.Dy() {
			t.Errorf("%s: %v, want %s", u, b, size)
		}
	}
}

// Every page template of the site has the slot for the web app's head tags,
// in its head: a new page without it could not be installed from and would
// show no icon. Read from the directory, so a new template is covered too.
func TestTemplatesHaveAppSlot(t *testing.T) {
	files, err := filepath.Glob("assets/*.html")
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		head, _, ok := strings.Cut(string(b), "</head>")
		if !ok || !strings.Contains(head, "<!--{{APP}}-->") {
			t.Errorf("%s: no <!--{{APP}}--> in its <head>", f)
		}
	}
}
