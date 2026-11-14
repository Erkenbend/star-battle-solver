package starbattle

import "testing"

func TestFileRating(t *testing.T) {
	for text, want := range map[string]int{
		"# generate ...\n# rating: 2 (4 lookahead steps, longest 5)\nAB\nBA\n": 2,
		"#rating:3\nAB\n":               0, // the key needs its space
		"# rating: 3\nAB\n":             3,
		"  # rating: 1 (rules alone)\n": 1,
		"# rating: 4\n":                 0,
		"# rating: 12\n":                0,
		"# rating:\n":                   0,
		"AB\nBA\n":                      0,
		"# name: Smiley\n":              0,
	} {
		if got := FileRating([]byte(text)); got != want {
			t.Errorf("FileRating(%q) = %d, want %d", text, got, want)
		}
	}
}
