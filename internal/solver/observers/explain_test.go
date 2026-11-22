package observers

import (
	"slices"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

func TestConfinementReason(t *testing.T) {
	// Regions B and C only in rows 0,1 (see confinement_test.go).
	s := board.MustParse(p5, `
		.....
		.....
		..X..
		.....
		.....`)
	marks := NewConfinement(p5, puzzle.Region, puzzle.Row, 2).Deduce(s)
	if len(marks) == 0 {
		t.Fatal("no marks")
	}
	want := "Farben B und C liegen nur noch in Zeilen 0 und 1 → alle anderen Felder der Zeilen 0 und 1 X"
	if marks[0].Reason != want {
		t.Fatalf("got %q\nwant %q", marks[0].Reason, want)
	}
	// The page highlights what the text names from Refs.
	if r := marks[0].Refs; r == nil || !slices.Equal(slices.Sorted(slices.Values(r.Regions)), []int{1, 2}) ||
		!slices.Equal(slices.Sorted(slices.Values(r.Rows)), []int{0, 1}) || len(r.Cols)+len(r.Cells) != 0 {
		t.Errorf("refs %+v, want regions B and C, rows 0 and 1", r)
	}
}

func TestEliminationReasons(t *testing.T) {
	units := p5.Units()
	reasons := func(u puzzle.Unit, state string) map[puzzle.Pos]string {
		out := map[puzzle.Pos]string{}
		for _, m := range (Elimination{Unit: u}).Deduce(board.MustParse(p5, state)) {
			out[m.Pos] = m.Reason
		}
		return out
	}
	empty := `
		.....
		.....
		.....
		.....
		.....`

	// Region E = (3,0),(4,0), vertical 2x1 in column 0.
	got := reasons(units[10+p5.Region(4, 0)], empty)
	for pos, want := range map[puzzle.Pos]string{
		{Row: 0, Col: 0}: "Farbe E liegt nur noch in Spalte 0 → alle anderen Felder der Spalte 0 X",
		{Row: 3, Col: 1}: "Die markierten Felder berühren jeweils alle freien Felder von Farbe E → X",
	} {
		if got[pos] != want {
			t.Errorf("%v: got %q\nwant %q", pos, got[pos], want)
		}
	}

	// Row 2 down to (2,3),(2,4), both in region A.
	got = reasons(units[2], `
		.....
		.....
		XXX..
		.....
		.....`)
	if want := "Zeile 2 hat nur noch freie Felder der Farbe A → alle anderen Felder der Farbe A X"; got[puzzle.Pos{Row: 4, Col: 4}] != want {
		t.Errorf("got %q\nwant %q", got[puzzle.Pos{Row: 4, Col: 4}], want)
	}
}
