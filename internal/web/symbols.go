package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// SymbolsFile is the catalog in a symbols directory: a JSON array of Symbol.
const SymbolsFile = "symbols.json"

// Kinds of symbols. SVG shapes and glyphs take the page's star colors (and
// turn red when wrong); emoji and sprites keep their own colors.
const (
	KindSVG    = "svg"    // Path: path data in a 100×100 box
	KindGlyph  = "glyph"  // Glyph: a character drawn as text (forced text style)
	KindEmoji  = "emoji"  // Glyph: an emoji of the device's font
	KindSprite = "sprite" // Src: an image file, Frames side by side if animated
)

// MaxSymbolFile is the size limit of an image file of the catalog.
const MaxSymbolFile = 256 << 10

// Symbol is a symbol players can choose for their stars (see /einstellungen).
// The page keeps a copy of the chosen one in the browser's storage, so it
// draws without asking the server.
type Symbol struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"` // shown to the player
	Kind   string  `json:"kind"`
	Path   string  `json:"path,omitempty"`
	Glyph  string  `json:"glyph,omitempty"`
	Src    string  `json:"src,omitempty"`    // file in the symbols directory
	URL    string  `json:"url,omitempty"`    // where the page loads Src, set on load
	Frames int     `json:"frames,omitempty"` // sprite: frames of 100×100 side by side
	FPS    int     `json:"fps,omitempty"`    // sprite: frames per second if animated
	Scale  float64 `json:"scale,omitempty"`  // size relative to the cell, default 1
}

// Symbols is the catalog of star symbols with its image files.
type Symbols struct {
	list  []Symbol
	files map[string]symbolFile // by Src
}

type symbolFile struct {
	data  []byte
	ctype string
	hash  string
}

var (
	symbolID   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	symbolPath = regexp.MustCompile(`^[MmLlHhVvCcSsQqTtAaZz0-9 ,.eE+-]+$`)
	symbolSrc  = regexp.MustCompile(`^[a-z0-9-]+\.(svg|png|webp)$`)
)

var symbolTypes = map[string]string{".svg": "image/svg+xml", ".png": "image/png", ".webp": "image/webp"}

// Variation selectors: text style (U+FE0E) and emoji style (U+FE0F).
const (
	textStyle  = "︎"
	emojiStyle = "️"
)

// LoadSymbols reads the catalog in dir of fsys (see SymbolsFile) and checks
// it: unique IDs, the fields each kind needs, image files present and small.
// Glyphs get the variation selector of their kind appended, so a glyph is not
// drawn as emoji and an emoji not as a plain character.
func LoadSymbols(fsys fs.FS, dir string) (*Symbols, error) {
	raw, err := fs.ReadFile(fsys, path.Join(dir, SymbolsFile))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var list []Symbol
	if err := dec.Decode(&list); err != nil {
		return nil, fmt.Errorf("%s: %w", SymbolsFile, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: no symbols", SymbolsFile)
	}
	s := &Symbols{files: map[string]symbolFile{}}
	ids := map[string]bool{}
	for i := range list {
		sym := &list[i]
		bad := func(format string, a ...any) error {
			return fmt.Errorf("%s: symbol %q: %s", SymbolsFile, sym.ID, fmt.Sprintf(format, a...))
		}
		if !symbolID.MatchString(sym.ID) {
			return nil, bad("id must be lower case letters, digits and dashes")
		}
		if ids[sym.ID] {
			return nil, bad("duplicate id")
		}
		ids[sym.ID] = true
		if strings.TrimSpace(sym.Name) == "" {
			return nil, bad("no name")
		}
		if sym.Scale < 0 || sym.Scale > 2 {
			return nil, bad("scale %g out of 0 to 2", sym.Scale)
		}
		if sym.URL != "" {
			return nil, bad("url is set by the server")
		}
		switch sym.Kind {
		case KindSVG:
			if !symbolPath.MatchString(sym.Path) {
				return nil, bad("path missing or not plain path data")
			}
		case KindGlyph, KindEmoji:
			g := strings.NewReplacer(textStyle, "", emojiStyle, "").Replace(sym.Glyph)
			if n := utf8.RuneCountInString(g); n == 0 || n > 8 {
				return nil, bad("glyph must be 1 to 8 characters")
			}
			if sym.Kind == KindGlyph {
				sym.Glyph = g + textStyle
			} else {
				sym.Glyph = g + emojiStyle
			}
		case KindSprite:
			if !symbolSrc.MatchString(sym.Src) {
				return nil, bad("src must be a .svg, .png or .webp file name")
			}
			if sym.Frames == 0 {
				sym.Frames = 1
			}
			if sym.Frames < 1 || sym.Frames > 64 {
				return nil, bad("frames %d out of 1 to 64", sym.Frames)
			}
			if sym.Frames > 1 && (sym.FPS < 1 || sym.FPS > 30) {
				return nil, bad("an animated sprite needs fps from 1 to 30")
			}
			f, ok := s.files[sym.Src]
			if !ok {
				data, err := fs.ReadFile(fsys, path.Join(dir, sym.Src))
				if err != nil {
					return nil, bad("%v", err)
				}
				if len(data) > MaxSymbolFile {
					return nil, bad("%s larger than %d bytes", sym.Src, MaxSymbolFile)
				}
				h := sha256.Sum256(data)
				f = symbolFile{data: data, ctype: symbolTypes[path.Ext(sym.Src)], hash: hex.EncodeToString(h[:])[:12]}
				s.files[sym.Src] = f
			}
			sym.URL = "/symbole/" + sym.Src + "?v=" + f.hash
		default:
			return nil, bad("unknown kind %q", sym.Kind)
		}
		if sym.Kind != KindSVG && sym.Path != "" || sym.Kind != KindGlyph && sym.Kind != KindEmoji && sym.Glyph != "" ||
			sym.Kind != KindSprite && (sym.Src != "" || sym.Frames != 0 || sym.FPS != 0) {
			return nil, bad("fields of another kind")
		}
	}
	s.list = list
	return s, nil
}

// List returns the symbols in catalog order.
func (s *Symbols) List() []Symbol { return s.list }

// SettingsPage is the data of the settings page.
type SettingsPage struct {
	Symbols []Symbol `json:"symbols"`
}

// WriteSettings renders the settings page for d to w.
func WriteSettings(w io.Writer, d SettingsPage) error {
	return render(w, settingsHTML, "Star Battle · Einstellungen", d, appJS, settingsJS)
}

// symbolRoutes adds the settings and the symbol catalog to mux:
//
//	GET /einstellungen      settings page: choose the star symbol
//	GET /symbole.json       the catalog
//	GET /symbole/{file}     an image file of the catalog; with ?v= its current
//	                        hash, cached for good
func symbolRoutes(mux *http.ServeMux, syms *Symbols) {
	mux.HandleFunc("GET /einstellungen", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, http.StatusOK, func(b io.Writer) error { return WriteSettings(b, SettingsPage{Symbols: syms.list}) })
	})
	mux.HandleFunc("GET /symbole.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(syms.list)
	})
	mux.HandleFunc("GET /symbole/{file}", func(w http.ResponseWriter, r *http.Request) {
		f, ok := syms.files[r.PathValue("file")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", f.ctype)
		h.Set("X-Content-Type-Options", "nosniff")
		// Opened on its own, an SVG must not run scripts or load anything.
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		h.Set("ETag", `"`+f.hash+`"`)
		if r.URL.Query().Get("v") == f.hash {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(f.data))
	})
}
