// Package web writes a puzzle and its solving trace as a self-contained,
// interactive HTML page: solve it yourself (with checks and hints taken from
// the trace) or step through the solver's explanation.
//
// The page is one file without external resources. Its HTML, CSS and
// JavaScript live in assets/ and are inlined together with the data as JSON.
// Every page with script gets dom.js (Dom, small DOM helpers) and logic.js
// (Logic, the pure functions), then its own script: on the puzzle page its UI
// (pageJS), app.js, the core, and one file per part of it.
package web

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/words"
)

//go:embed assets/page.html
var pageHTML string

//go:embed assets/style.css
var styleCSS string

var (
	//go:embed assets/dom.js
	domJS string
	//go:embed assets/logic.js
	logicJS string

	sharedJS = domJS + "\n" + logicJS
)

// The puzzle page's UI. app.js holds the core and boot(), which creates the
// other parts with the page object P (see there); they are function
// declarations, so the order only matters for reading.
var (
	//go:embed assets/storage.js
	storageJS string
	//go:embed assets/help.js
	helpJS string
	//go:embed assets/hypothesis.js
	hypothesisJS string
	//go:embed assets/watch.js
	watchJS string
	//go:embed assets/tutorial.js
	tutorialJS string
	//go:embed assets/round.js
	roundJS string
	//go:embed assets/coop.js
	coopJS string
	//go:embed assets/race.js
	raceJS string
	//go:embed assets/app.js
	appJS string

	pageJS = strings.Join([]string{storageJS, helpJS, hypothesisJS, watchJS, tutorialJS, roundJS, coopJS, raceJS, appJS}, "\n")
)

//go:embed assets/party.html
var partyHTML string

//go:embed assets/party.js
var partyJS string

//go:embed assets/qr.js
var qrJS string // only the party page draws QR codes

//go:embed assets/library.html
var libraryHTML string

//go:embed assets/library.js
var libraryJS string

//go:embed assets/error.html
var errorHTML string

//go:embed assets/settings.html
var settingsHTML string

//go:embed assets/settings.js
var settingsJS string

// Data is everything the page needs, serialized as JSON.
type Data struct {
	Title    string  `json:"title"`
	N        int     `json:"n"`
	Regions  [][]int `json:"regions"`
	Letters  string  `json:"letters"`            // letter of each region index
	Solution []int   `json:"solution,omitempty"` // star column per row, if unique
	Status   string  `json:"status"`             // solved / stuck / contradiction
	Error    string  `json:"error,omitempty"`
	Stats    Stats   `json:"stats"`
	Steps    []Step  `json:"steps"`

	// Set by the server (starbattle serve): the page shows forms for new puzzles.
	Served bool   `json:"served,omitempty"`
	Text   string `json:"text,omitempty"` // the puzzle in the text format
	MinN   int    `json:"minN,omitempty"` // sizes offered for a new puzzle
	MaxN   int    `json:"maxN,omitempty"`

	// Set for a round of a party (see party_server.go).
	Party *PartyInfo `json:"party,omitempty"`

	// Set for a puzzle of the library (see library.go): no help while solving.
	Library *LibraryInfo `json:"library,omitempty"`

	// Set for the tutorial (see tutorial.go).
	Tutorial *Tutorial `json:"tutorial,omitempty"`
}

// PartyInfo identifies the party round a page belongs to.
type PartyInfo struct {
	Code   string `json:"code"`
	Round  int    `json:"round"`
	Mode   string `json:"mode"`             // party.ModeVersus, ModeCoop or ModeClock
	Puzzle int    `json:"puzzle,omitempty"` // ModeClock: which of the round's puzzles, from 1
	Lives  int    `json:"lives,omitempty"`  // ModeVersus: lives per puzzle, 0 unlimited
}

// Stats summarizes the search effort.
type Stats struct {
	Lookahead int `json:"lookahead"`
	Guesses   int `json:"guesses"`
}

// Step is one deduction of the trace.
type Step struct {
	Rule       string      `json:"rule"` // observer type, e.g. "Confinement", or "Hypothesis"
	Source     string      `json:"source"`
	Level      int         `json:"level"`
	Reason     string      `json:"reason"`
	Refs       *Refs       `json:"refs,omitempty"` // what Reason names
	Marks      []Mark      `json:"marks"`
	Hypothesis *Hypothesis `json:"hypothesis,omitempty"`
}

// Refs are the units and cells an explanation names (words.Refs): the page
// highlights them and shows a color swatch for each named region, without
// reading the German text.
type Refs struct {
	Cells   [][2]int `json:"cells,omitempty"`
	Rows    []int    `json:"rows,omitempty"`
	Cols    []int    `json:"cols,omitempty"`
	Regions []int    `json:"regions,omitempty"`
}

func refs(r *words.Refs) *Refs {
	if r.Empty() {
		return nil
	}
	out := &Refs{Rows: r.Rows, Cols: r.Cols, Regions: r.Regions}
	for _, q := range r.Cells {
		out.Cells = append(out.Cells, [2]int{q.Row, q.Col})
	}
	return out
}

// Mark is [row, col, 1 for a star / 0 for a cross], compact in JSON.
type Mark [3]int

// Hypothesis is the refuted chain behind a lookahead or guess step.
type Hypothesis struct {
	Steps         []Step   `json:"steps"`
	Contradiction string   `json:"contradiction"`
	Refs          *Refs    `json:"refs,omitempty"`  // what Contradiction names
	Cells         [][2]int `json:"cells,omitempty"` // cells that show the contradiction
}

// Build converts a puzzle, its unique solution (nil if unknown) and a solver
// result into page data.
func Build(title string, p *puzzle.Puzzle, solution []int, res solver.Result) Data {
	letters := make([]byte, p.N)
	for i := range letters {
		letters[i] = puzzle.RegionLetter(i)
	}
	d := Data{
		Title:    title,
		N:        p.N,
		Regions:  p.Regions,
		Letters:  string(letters),
		Solution: solution,
		Status:   res.Status.String(),
		Stats:    Stats{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses},
		Steps:    steps(res.Trace),
	}
	if res.Err != nil {
		d.Error = res.Err.Error()
	}
	return d
}

func steps(trace []solver.Step) []Step {
	out := make([]Step, len(trace))
	for i, st := range trace {
		rule, _, _ := strings.Cut(st.Source, "[")
		s := Step{Rule: rule, Source: st.Source, Level: st.Level, Reason: st.Reason.Text(), Refs: refs(st.Reason.Refs()), Marks: []Mark{}}
		for _, m := range st.Marks {
			star := 0
			if m.Kind == board.Star {
				star = 1
			}
			s.Marks = append(s.Marks, Mark{m.Pos.Row, m.Pos.Col, star})
		}
		if h := st.Hypothesis; h != nil {
			hy := &Hypothesis{Steps: steps(h.Steps)}
			if h.Contradiction != nil {
				hy.Contradiction = h.Contradiction.Error()
				if ce, ok := errors.AsType[*board.ContradictionError](h.Contradiction); ok {
					hy.Refs = refs(ce.Refs)
					for _, q := range ce.Cells {
						hy.Cells = append(hy.Cells, [2]int{q.Row, q.Col})
					}
				}
			}
			s.Hypothesis = hy
		}
		out[i] = s
	}
	return out
}

// Write renders the page for d to w. Party rounds get the party script too
// (before the page's, which uses it), served pages the links of the web app
// (install.go).
func Write(w io.Writer, d Data) error {
	script := pageJS
	if d.Party != nil {
		script = partyJS + "\n" + pageJS
	}
	head := ""
	if d.Served {
		head = appHead
	}
	return render(w, pageHTML, d.Title, d, script, head)
}

// PartyPage is the data of the party page: creating or joining a party
// (Code empty), or its lobby.
type PartyPage struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
	MinN  int    `json:"minN"`
	MaxN  int    `json:"maxN"`
}

// WriteParty renders the party page for d to w.
func WriteParty(w io.Writer, d PartyPage) error {
	title := "Star Battle · Zusammen spielen"
	if d.Code != "" {
		title = "Star Battle · Party " + d.Code
	}
	return render(w, partyHTML, title, d, qrJS+"\n"+partyJS, appHead)
}

// Link is a further way out of an error page, next to the start page.
type Link struct{ Href, Text string }

// WriteError renders an error page: the message, a button to the start page
// and the given links, below the site's navigation.
func WriteError(w io.Writer, title, msg string, links ...Link) error {
	var lb strings.Builder
	for _, l := range links {
		fmt.Fprintf(&lb, "\n      <a class=\"button\" href=\"%s\">%s</a>", html.EscapeString(l.Href), html.EscapeString(l.Text))
	}
	_, err := strings.NewReplacer(
		"{{TITLE}}", html.EscapeString(title),
		"<!--{{APP}}-->", appHead,
		"/*{{STYLE}}*/", styleCSS,
		"{{MESSAGE}}", html.EscapeString(msg),
		"{{LINKS}}", lb.String(),
	).WriteString(w, errorHTML)
	return err
}

// ServePage sends a page of the site built from tmpl, a template like
// assets/library.html: with the site's style, the links of the web app if it
// has their placeholder, d as its data, the shared scripts and script (the
// page's own, e.g. the designer's).
func ServePage(w http.ResponseWriter, tmpl, title string, d any, script string) {
	writeHTML(w, http.StatusOK, func(b io.Writer) error { return render(b, tmpl, title, d, script, appHead) })
}

// render fills tmpl: the title, the style, the head tags of the web app
// (appHead on served pages, else empty), d as JSON, and the scripts: the
// shared ones (dom.js, logic.js) in /*{{SHARED}}*/, which every template has
// before the page's own script in /*{{SCRIPT}}*/.
func render(w io.Writer, tmpl, title string, d any, script, head string) error {
	data, err := json.Marshal(d) // escapes <, > and &, so it is safe inside <script>
	if err != nil {
		return err
	}
	page := strings.NewReplacer(
		"{{TITLE}}", html.EscapeString(title),
		"<!--{{APP}}-->", head,
		"/*{{STYLE}}*/", styleCSS,
		"/*{{DATA}}*/null", string(data),
		"/*{{SCRIPT}}*/", script,
		"/*{{SHARED}}*/", sharedJS,
	).Replace(tmpl)
	if strings.Contains(page, "/*{{") {
		return fmt.Errorf("web: unreplaced placeholder in page template")
	}
	_, err = io.WriteString(w, page)
	return err
}
