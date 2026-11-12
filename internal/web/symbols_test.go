package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"starbattle"
)

func TestLoadSymbolsCatalog(t *testing.T) {
	syms, err := LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, s := range syms.List() {
		kinds[s.Kind] = true
		if s.Kind == KindSprite && s.Frames > 1 {
			kinds["animated"] = true
		}
	}
	// The catalog shows an example of every kind.
	for _, k := range []string{KindSVG, KindGlyph, KindEmoji, KindSprite, "animated"} {
		if !kinds[k] {
			t.Errorf("no symbol of kind %s", k)
		}
	}
	if first := syms.List()[0]; first.ID != "star" {
		t.Errorf("first symbol %q, want the default star", first.ID)
	}
}

func TestLoadSymbolsNormalizes(t *testing.T) {
	fsys := fstest.MapFS{
		"s/symbols.json": {Data: []byte(`[
			{"id": "club", "name": "Kreuz", "kind": "glyph", "glyph": "♣️"},
			{"id": "clover", "name": "Klee", "kind": "emoji", "glyph": "🍀"},
			{"id": "img", "name": "Bild", "kind": "sprite", "src": "a.svg"}
		]`)},
		"s/a.svg": {Data: []byte("<svg/>")},
	}
	syms, err := LoadSymbols(fsys, "s")
	if err != nil {
		t.Fatal(err)
	}
	l := syms.List()
	if l[0].Glyph != "♣︎" || l[1].Glyph != "🍀️" {
		t.Errorf("glyphs %q %q, want text and emoji style", l[0].Glyph, l[1].Glyph)
	}
	if l[2].Frames != 1 || !strings.HasPrefix(l[2].URL, "/symbole/a.svg?v=") {
		t.Errorf("sprite frames %d url %q", l[2].Frames, l[2].URL)
	}
}

func TestLoadSymbolsRejects(t *testing.T) {
	for name, entry := range map[string]string{
		"bad id":         `{"id": "Stern", "name": "x", "kind": "svg", "path": "M0 0Z"}`,
		"no name":        `{"id": "a", "name": "", "kind": "svg", "path": "M0 0Z"}`,
		"markup in path": `{"id": "a", "name": "x", "kind": "svg", "path": "M0 0\"/><script>"}`,
		"unknown kind":   `{"id": "a", "name": "x", "kind": "gif"}`,
		"long glyph":     `{"id": "a", "name": "x", "kind": "glyph", "glyph": "abcdefghij"}`,
		"missing file":   `{"id": "a", "name": "x", "kind": "sprite", "src": "none.svg"}`,
		"path in src":    `{"id": "a", "name": "x", "kind": "sprite", "src": "../a.svg"}`,
		"gif":            `{"id": "a", "name": "x", "kind": "sprite", "src": "a.gif"}`,
		"no fps":         `{"id": "a", "name": "x", "kind": "sprite", "src": "a.svg", "frames": 4}`,
		"mixed fields":   `{"id": "a", "name": "x", "kind": "svg", "path": "M0 0Z", "glyph": "x"}`,
		"url given":      `{"id": "a", "name": "x", "kind": "sprite", "src": "a.svg", "url": "https://example.com/a.svg"}`,
		"unknown field":  `{"id": "a", "name": "x", "kind": "svg", "path": "M0 0Z", "color": "red"}`,
	} {
		fsys := fstest.MapFS{
			"s/symbols.json": {Data: []byte("[" + entry + "]")},
			"s/a.svg":        {Data: []byte("<svg/>")},
			"s/a.gif":        {Data: []byte("GIF89a")},
		}
		if _, err := LoadSymbols(fsys, "s"); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	dup := fstest.MapFS{"s/symbols.json": {Data: []byte(`[
		{"id": "a", "name": "x", "kind": "svg", "path": "M0 0Z"},
		{"id": "a", "name": "y", "kind": "svg", "path": "M0 0Z"}]`)}}
	if _, err := LoadSymbols(dup, "s"); err == nil {
		t.Error("duplicate id: no error")
	}
	big := fstest.MapFS{
		"s/symbols.json": {Data: []byte(`[{"id": "a", "name": "x", "kind": "sprite", "src": "a.png"}]`)},
		"s/a.png":        {Data: make([]byte, MaxSymbolFile+1)},
	}
	if _, err := LoadSymbols(big, "s"); err == nil {
		t.Error("large file: no error")
	}
}

func TestSymbolRoutes(t *testing.T) {
	syms, err := LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(testSolve, nil, nil, nil, syms)

	rec := get(h, "/einstellungen")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="symbol-list"`) {
		t.Fatalf("/einstellungen: %d", rec.Code)
	}

	rec = get(h, "/symbole.json")
	var list []Symbol
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != len(syms.List()) {
		t.Fatalf("/symbole.json: %v, %d symbols", err, len(list))
	}

	var sprite *Symbol
	for i := range list {
		if list[i].Kind == KindSprite {
			sprite = &list[i]
			break
		}
	}
	rec = get(h, sprite.URL)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("%s: %d %s", sprite.URL, rec.Code, rec.Header().Get("Content-Type"))
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("%s: Cache-Control %q, want immutable", sprite.URL, cc)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("%s: Content-Security-Policy %q", sprite.URL, csp)
	}
	// Without the current hash it is served too, but not cached for good.
	rec = get(h, "/symbole/"+sprite.Src+"?v=old")
	if rec.Code != http.StatusOK || strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("old hash: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	// Revalidation by ETag.
	req := httptest.NewRequest("GET", sprite.URL, nil)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: %d, want 304", rec.Code)
	}
	for _, p := range []string{"/symbole/none.svg", "/symbole/symbols.json", "/symbole/cat.py"} {
		if rec := get(h, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, rec.Code)
		}
	}

	// The designer's play page links to the settings, so it serves them too.
	dh := DesignerHandler(t.TempDir(), testSolve, syms)
	for _, p := range []string{"/einstellungen", "/symbole.json", sprite.URL} {
		if rec := get(dh, p); rec.Code != http.StatusOK {
			t.Errorf("designer %s: %d", p, rec.Code)
		}
	}

	// Without a catalog there are no settings.
	if rec := get(Handler(testSolve, nil, nil, nil, nil), "/einstellungen"); rec.Code != http.StatusNotFound {
		t.Errorf("no catalog: /einstellungen %d", rec.Code)
	}
}
