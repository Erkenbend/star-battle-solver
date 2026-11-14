package web

import (
	"fmt"
	"net/http"
	"strings"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

// The tutorial (/anleitung) is the normal play page with a small fixed puzzle.
// It guides through the first moves (a star by double click, crosses by click
// and by dragging), lets the player solve the rest and then points at the
// page's buttons one after the other. app.js sends new visitors of a solo page
// there once; the "?" in the navigation leads there any time.

// tutorialPuzzle has a one-cell region A, so the first star is obvious, and
// after crossing around it region D has a single free cell.
const tutorialPuzzle = `BBBCC
BBCCC
ADDDC
DDDEE
DEEEE
`

// Tutorial is the content of the tutorial, shown by app.js.
type Tutorial struct {
	Steps []TutorialStep `json:"steps"`
	Tour  []TourStop     `json:"tour"`
}

// TutorialStep is one guided move. It is done once the cells in Stars hold a
// star and those in Crosses a cross, or for Solve once the puzzle is solved.
type TutorialStep struct {
	Action  string   `json:"action"` // the gesture, as a heading
	Text    string   `json:"text"`
	Stars   [][2]int `json:"stars,omitempty"`
	Crosses [][2]int `json:"crosses,omitempty"`
	Solve   bool     `json:"solve,omitempty"`
}

// TourStop points at an element of the page (by id) with a short text.
type TourStop struct {
	Target string `json:"target"`
	Text   string `json:"text"`
}

var tutorial = Tutorial{
	Steps: []TutorialStep{
		{
			Action: "Doppelklick: Stern",
			Text:   "In jede Zeile, jede Spalte und jede Farbe gehört genau ein Stern. Farbe A hat nur ein Feld, also steht der Stern dort.",
			Stars:  [][2]int{{2, 0}},
		},
		{
			Action:  "Klick: Kreuz",
			Text:    "Sterne berühren sich nicht, auch nicht schräg. Kreuze die beiden markierten Felder.",
			Crosses: [][2]int{{1, 1}, {3, 1}},
		},
		{
			Action:  "Ziehen: viele Kreuze",
			Text:    "In Zeile 2 passt kein weiterer Stern. Halte gedrückt und ziehe über die markierten Felder.",
			Crosses: [][2]int{{2, 1}, {2, 2}, {2, 3}, {2, 4}},
		},
		{
			Action:  "Ziehen: viele Kreuze",
			Text:    "Genauso in Spalte 0.",
			Crosses: [][2]int{{0, 0}, {1, 0}, {3, 0}, {4, 0}},
		},
		{
			Action: "Jetzt du",
			Text:   "In Farbe D ist nur noch ein Feld frei. Löse den Rest selbst; „Hinweis“ hilft, wenn du feststeckst.",
			Solve:  true,
		},
	},
	Tour: []TourStop{
		{"nav-party", "Zusammen spielen: Partys mit anderen, gegeneinander, miteinander oder gegen die Uhr."},
		{"nav-library", "Bibliothek: Motive und besonders schwere Puzzles."},
		{"hyp", "Annahme: Steckst du fest, setz probehalber einen Stern und schau, ob er zu einem Widerspruch führt."},
		{"tab-watch", "Lösungsweg: Der Solver zeigt Schritt für Schritt, wie er das Puzzle löst, mit Begründung."},
		{"nav-settings", "Einstellungen: Nachbarn automatisch kreuzen, Fat-Finger-Schutz, ein eigenes Symbol statt ★."},
		{"nav-help", "Diese Anleitung findest du jederzeit hier."},
	},
}

// tutorialData builds the page data of the tutorial.
func tutorialData(solve SolveFunc) (Data, error) {
	p, err := puzzle.Parse(strings.NewReader(tutorialPuzzle))
	if err != nil {
		return Data{}, err
	}
	sols := bruteforce.Solve(p, 2)
	if len(sols) != 1 {
		return Data{}, fmt.Errorf("web: tutorial puzzle has %d solutions, want 1", len(sols))
	}
	res, err := solve(p)
	if err != nil {
		return Data{}, err
	}
	d := Build("Star Battle · Anleitung", p, sols[0], res)
	d.Served = true
	d.Tutorial = &tutorial
	return d, nil
}

func tutorialRoutes(mux *http.ServeMux, solve SolveFunc) {
	mux.HandleFunc("GET /anleitung", func(w http.ResponseWriter, r *http.Request) {
		d, err := tutorialData(solve)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writePage(w, d)
	})
}
