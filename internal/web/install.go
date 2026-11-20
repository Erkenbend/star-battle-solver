package web

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"starbattle/internal/icon"
)

// Installing the site as a web app (decision 26): a manifest and icons, and
// the head tags that link them on every served page. No service worker, so
// nothing works offline; the app is the site in a window of its own.

// iconFiles are the icons, drawn by package icon (go test ./internal/icon
// -update writes them).
//
//go:embed assets/icons/*.png
var iconFiles embed.FS

// appHead goes into the <head> of every served page (the placeholder
// <!--{{APP}}--> of the templates); a page saved with `starbattle html` gets
// nothing, it has no server to fetch these from. The manifest is fetched with
// credentials: without them a site behind basic auth answers 401 and the
// page is not installable.
const appHead = `<link rel="manifest" href="/manifest.webmanifest" crossorigin="use-credentials">
<link rel="icon" href="/icon-32.png" sizes="32x32" type="image/png">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<meta name="theme-color" content="#f6f5f2" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#17171a" media="(prefers-color-scheme: dark)">`

// Manifest is the web app manifest.
type Manifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	Lang            string         `json:"lang"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []ManifestIcon `json:"icons"`
}

// ManifestIcon is an icon of the manifest.
type ManifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

func manifest() Manifest {
	m := Manifest{
		ID:              "/",
		Name:            "Star Battle",
		ShortName:       "Star Battle",
		Description:     "Star Battle lösen, allein oder zusammen, mit einem Lösungsweg, der jeden Schritt erklärt.",
		Lang:            "de",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: "#f6f5f2", // --bg of style.css, light theme
		ThemeColor:      "#f6f5f2",
	}
	for _, ic := range icon.All {
		if ic.Purpose != "" {
			m.Icons = append(m.Icons, ManifestIcon{Src: "/" + ic.File, Sizes: sizes(ic.Size), Type: "image/png", Purpose: ic.Purpose})
		}
	}
	return m
}

func sizes(n int) string { return strconv.Itoa(n) + "x" + strconv.Itoa(n) }

// installRoutes serves the manifest and the icons. Both change only with a
// new binary; a day in the browser's cache is short enough for that.
func installRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "max-age=86400")
		json.NewEncoder(w).Encode(manifest())
	})
	for _, ic := range icon.All {
		b, err := iconFiles.ReadFile("assets/icons/" + ic.File)
		if err != nil {
			panic(err) // an icon of icon.All without its file: go test ./internal/icon -update
		}
		mux.HandleFunc("GET /"+ic.File, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "max-age=86400")
			w.Write(b)
		})
	}
}
