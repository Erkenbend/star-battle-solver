package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestTutorialContent(t *testing.T) {
	d, err := tutorialData(testSolve)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "solved" || !d.Served || d.Tutorial == nil {
		t.Fatalf("status %s, served %v, tutorial %v", d.Status, d.Served, d.Tutorial != nil)
	}
	steps := d.Tutorial.Steps
	if len(steps) == 0 || !steps[len(steps)-1].Solve {
		t.Fatal("the last step must be solving the rest")
	}
	in := func(p [2]int) bool { return p[0] >= 0 && p[0] < d.N && p[1] >= 0 && p[1] < d.N }
	for k, s := range steps {
		if s.Action == "" || s.Text == "" {
			t.Errorf("step %d lacks a text", k)
		}
		if !s.Solve && len(s.Stars)+len(s.Crosses) == 0 {
			t.Errorf("step %d asks for nothing", k)
		}
		// Every step agrees with the solution, so following them never blocks it.
		for _, p := range s.Stars {
			if !in(p) || d.Solution[p[0]] != p[1] {
				t.Errorf("step %d: star %v is not in the solution", k, p)
			}
		}
		for _, p := range s.Crosses {
			if !in(p) || d.Solution[p[0]] == p[1] {
				t.Errorf("step %d: cross %v is on a star of the solution", k, p)
			}
		}
	}
	// The tour points at elements of the play page.
	for _, s := range d.Tutorial.Tour {
		if !strings.Contains(pageHTML, `id="`+s.Target+`"`) {
			t.Errorf("tour target %q is not on the page", s.Target)
		}
	}
}

func TestTutorialServed(t *testing.T) {
	h := Handler(testSolve, nil, nil, nil, nil)
	rec := get(h, "/anleitung")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	d := pageData(t, rec.Body.String())
	if d.Tutorial == nil || len(d.Tutorial.Steps) != len(tutorial.Steps) || d.N != 5 {
		t.Fatalf("bad tutorial data: n=%d %+v", d.N, d.Tutorial)
	}
	// Every page with the site's navigation links to it.
	for name, tmpl := range map[string]string{"page": pageHTML, "party": partyHTML, "library": libraryHTML, "settings": settingsHTML, "error": errorHTML} {
		if !strings.Contains(tmpl, `href="/anleitung"`) {
			t.Errorf("%s.html has no link to the tutorial", name)
		}
	}
}
