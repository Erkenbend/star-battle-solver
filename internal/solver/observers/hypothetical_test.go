package observers

import (
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// testdata/hard/n05_s029.txt and the state where the base rules get stuck.
var (
	hard5 = puzzle.MustParse(`
		CCAAA
		CCADA
		EDDDA
		EDEBB
		EEEEB`)
	hard5Stuck = `
		.....
		..XX.
		X...X
		.XX..
		...X.`
)

func TestHard5IsStuckState(t *testing.T) {
	got := solver.RunSync(board.New(hard5), AllWith(hard5, BaseConfig))
	if got.Status != solver.Stuck || !got.State.Equal(board.MustParse(hard5, hard5Stuck)) {
		t.Fatalf("base rules end in %v:\n%s", got.Status, got.State)
	}
}

func TestForcedPartner(t *testing.T) {
	units := hard5.Units()
	at := func(r, c, depth int) solver.Observer {
		return ForcedPartner{Cell: puzzle.Pos{Row: r, Col: c}, Depth: depth, units: units}
	}
	// (0,4): leaves only (3,3) in region B; together they empty column 2.
	expectDeduce(t, hard5, at(0, 4, 1), hard5Stuck, `
		....X
		..XX.
		X...X
		.XX..
		...X.`)
	// (0,0): no partner leads to an empty unit (DeadCells' case, see below).
	expectDeduce(t, hard5, at(0, 0, 1), hard5Stuck, hard5Stuck)

	marks := at(0, 4, 1).Deduce(board.MustParse(hard5, hard5Stuck))
	want := "Ein Stern auf (0,4) ließe in Farbe B nur (3,3) übrig. Beide Sterne zusammen würden Spalte 2 komplett streichen → X"
	if len(marks) != 1 || marks[0].Reason != want {
		t.Fatalf("got %v\nwant %q", marks, want)
	}
}

func TestDeadCells(t *testing.T) {
	units := hard5.Units()
	at := func(r, c, max int) solver.Observer {
		return DeadCells{Cell: puzzle.Pos{Row: r, Col: c}, MaxCells: max, units: units}
	}
	// (0,0): row 3 keeps (3,3) and (3,4), each of which would empty a unit.
	expectDeduce(t, hard5, at(0, 0, 2), hard5Stuck, `
		X....
		..XX.
		X...X
		.XX..
		...X.`)
	// (2,1) stays possible.
	expectDeduce(t, hard5, at(2, 1, 2), hard5Stuck, hard5Stuck)

	for _, tt := range []struct {
		r, c int
		want string
	}{
		{0, 0, "Nach einem Stern auf (0,0) bleiben in Zeile 3 nur (3,3) und (3,4). Ein Stern auf (3,3) würde Spalte 2 streichen, einer auf (3,4) würde Zeile 1 streichen → X"},
		{1, 1, "Nach einem Stern auf (1,1) bleiben in Spalte 0 nur (3,0) und (4,0). Ein Stern auf (3,0) oder (4,0) würde Spalte 2 streichen → X"},
	} {
		marks := at(tt.r, tt.c, 2).Deduce(board.MustParse(hard5, hard5Stuck))
		if len(marks) != 1 || marks[0].Reason != tt.want {
			t.Errorf("got %v\nwant %q", marks, tt.want)
		}
	}
}

func TestForcedPartnerDepth(t *testing.T) {
	units := hard5.Units()
	s := board.MustParse(hard5, hard5Stuck)
	q := puzzle.Pos{Row: 0, Col: 2}
	if marks := (ForcedPartner{Cell: q, Depth: 1, units: units}).Deduce(s); len(marks) != 0 {
		t.Fatalf("depth 1 should not suffice for (0,2): %v", marks)
	}
	marks := ForcedPartner{Cell: q, Depth: 2, units: units}.Deduce(s)
	want := "Ein Stern auf (0,2) ließe in Zeile 1 nur (1,0) übrig, dann in Zeile 2 nur (2,3) übrig. Alle diese Sterne zusammen würden Zeile 3 komplett streichen → X"
	if len(marks) != 1 || marks[0].Reason != want {
		t.Fatalf("got %v\nwant %q", marks, want)
	}
}

// p5 layout (see unitfinish_test.go):
//
//	  01234
//	0 DBBBB
//	1 DDBCC
//	2 DDBAA
//	3 EDDAA
//	4 EAAAA
func TestPigeonhole(t *testing.T) {
	units := p5.Units()
	at := func(r, c, maxK int) Pigeonhole {
		return Pigeonhole{Cell: puzzle.Pos{Row: r, Col: c}, MaxK: maxK, units: units}
	}
	rows34 := `
		.....
		.....
		.....
		.X.XX
		.X.XX`
	// (0,2) crosses column 2: rows 3 and 4 keep only column 0.
	expectDeduce(t, p5, at(0, 2, 2), rows34, `
		..X..
		.....
		.....
		.X.XX
		.X.XX`)
	marks := at(0, 2, 2).Deduce(board.MustParse(p5, rows34))
	want := "Ein Stern auf (0,2) ließe Zeilen 3 und 4 nur noch Platz in Spalte 0, zu wenig für 2 Sterne → X"
	if len(marks) != 1 || marks[0].Reason != want {
		t.Fatalf("got %v\nwant %q", marks, want)
	}
	// Switched off below k=2.
	expectDeduce(t, p5, at(0, 2, 1), rows34, rows34)
	// (0,4) would empty region C: Elimination's case, not this rule's.
	expectDeduce(t, p5, at(0, 4, 2), `
		.....
		.....
		.....
		.XXX.
		.XXX.`, `
		.....
		.....
		.....
		.XXX.
		.XXX.`)
	// (1,4) leaves rows 2 and 4 only column 0, but they are not adjacent (row 3
	// is open in between), and no other set of units is short of room.
	nonAdjacent := `
		.....
		.....
		.XXX.
		.....
		.XXX.`
	expectDeduce(t, p5, at(1, 4, 2), nonAdjacent, nonAdjacent)
	after, _ := assume(board.MustParse(p5, nonAdjacent), puzzle.Pos{Row: 1, Col: 4})
	if emptyUnit(after, units) != nil {
		t.Fatal("test case should not empty a unit")
	}
	var rows []puzzle.Unit
	for _, u := range units {
		if u.Kind == puzzle.Row {
			rows = append(rows, u)
		}
	}
	found := false
	confinedSets(openUnits(after, rows, puzzle.Col), 2, 1, false, func(_, _ map[int]bool) bool { found = true; return false })
	if !found {
		t.Fatal("with any subset of rows the case should apply")
	}
}
