// Package web writes a puzzle and its solving trace as a self-contained,
// interactive HTML page: solve it yourself (with checks and hints taken from
// the trace) or step through the solver's explanation.
//
// The page is one file without external resources. Its HTML, CSS and
// JavaScript live in assets/ and are inlined together with the data as JSON.
package web

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

//go:embed assets/page.html
var pageHTML string

//go:embed assets/style.css
var styleCSS string

//go:embed assets/app.js
var appJS string

//go:embed assets/party.html
var partyHTML string

//go:embed assets/party.js
var partyJS string

//go:embed assets/library.html
var libraryHTML string

//go:embed assets/library.js
var libraryJS string

//go:embed assets/designer.html
var designerHTML string

//go:embed assets/designer.js
var designerJS string

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

	// Set for a round of a party (see party.go).
	Party *PartyInfo `json:"party,omitempty"`

	// Set for a puzzle of the library (see library.go): no help while solving.
	Library *LibraryInfo `json:"library,omitempty"`
}

// PartyInfo identifies the party round a page belongs to.
type PartyInfo struct {
	Code  string `json:"code"`
	Round int    `json:"round"`
	Mode  string `json:"mode"` // ModeVersus or ModeCoop
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
	Marks      []Mark      `json:"marks"`
	Hypothesis *Hypothesis `json:"hypothesis,omitempty"`
}

// Mark is [row, col, 1 for a star / 0 for a cross], compact in JSON.
type Mark [3]int

// Hypothesis is the refuted chain behind a lookahead or guess step.
type Hypothesis struct {
	Steps         []Step   `json:"steps"`
	Contradiction string   `json:"contradiction"`
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
		s := Step{Rule: rule, Source: st.Source, Level: st.Level, Reason: st.Reason, Marks: []Mark{}}
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

// Write renders the page for d to w. Party rounds get the party script too.
func Write(w io.Writer, d Data) error {
	party := ""
	if d.Party != nil {
		party = partyJS
	}
	return render(w, pageHTML, d.Title, d, appJS, party)
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
	return render(w, partyHTML, title, d, "", partyJS)
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
		"/*{{STYLE}}*/", styleCSS,
		"{{MESSAGE}}", html.EscapeString(msg),
		"{{LINKS}}", lb.String(),
	).WriteString(w, errorHTML)
	return err
}

func render(w io.Writer, tmpl, title string, d any, app, party string) error {
	data, err := json.Marshal(d) // escapes <, > and &, so it is safe inside <script>
	if err != nil {
		return err
	}
	page := strings.NewReplacer(
		"{{TITLE}}", html.EscapeString(title),
		"/*{{STYLE}}*/", styleCSS,
		"/*{{DATA}}*/null", string(data),
		"/*{{PARTY}}*/", party,
		"/*{{APP}}*/", app,
	).Replace(tmpl)
	if strings.Contains(page, "/*{{") {
		return fmt.Errorf("web: unreplaced placeholder in page template")
	}
	_, err = io.WriteString(w, page)
	return err
}
