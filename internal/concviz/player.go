package concviz

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed player.tmpl.html
var playerTmpl string

type playerStory struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Source string       `json:"source"`
	Total  float64      `json:"total"`
	SVG    string       `json:"svg"`
	Steps  []playerStep `json:"steps"`
}

type playerStep struct {
	T0      float64  `json:"t0"`
	T1      float64  `json:"t1"`
	Caption string   `json:"caption"`
	Ops     []string `json:"ops"`
}

// Player is a self-contained HTML page that plays the stories' scenes step
// by step: play, pause, step forward and back, scrub.
func Player(stories []*Story) []byte {
	var data []playerStory
	for _, s := range stories {
		t0, t1, total := s.timeline()
		ps := playerStory{ID: s.ID(), Title: s.Title, Source: s.Source, Total: round3(total), SVG: string(s.Scene())}
		for i := 1; i < len(s.frames); i++ {
			ops := s.frames[i].ops
			if ops == nil {
				ops = []string{} // a step in which nothing happens: [], not null
			}
			ps.Steps = append(ps.Steps, playerStep{T0: round3(t0[i]), T1: round3(t1[i]), Caption: s.frames[i].caption, Ops: ops})
		}
		data = append(data, ps)
	}
	js, err := json.Marshal(data) // escapes <, > and &, so it is safe inside <script>
	if err != nil {
		panic(err)
	}
	return []byte(strings.Replace(playerTmpl, "/*DATA*/", string(js), 1))
}

// ID is the story's file name without extension, the anchor in the player.
func (s *Story) ID() string { return strings.TrimSuffix(s.File, ".svg") }

func round3(f float64) float64 { return float64(int64(f*1000+0.5)) / 1000 }
