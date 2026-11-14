package puzzle

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRoundTrip(t *testing.T) {
	const text = "AAB\nACB\nCCB\n"
	p, err := Parse(strings.NewReader("# comment\n\n A A B\nACB\n\tCCB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.String(); got != text {
		t.Fatalf("String() = %q, want %q", got, text)
	}
	if p.Region(1, 1) != 2 || RegionLetter(2) != 'C' {
		t.Fatalf("region (1,1) = %d, want 2 (C)", p.Region(1, 1))
	}
}

func TestParseRenumbers(t *testing.T) {
	// Not the first N letters of the alphabet: regions in order of first appearance.
	p := MustParse("xxz\nxyz\nyyz\n")
	if got, want := p.String(), "AAB\nACB\nCCB\n"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestParseErrors(t *testing.T) {
	for name, text := range map[string]string{
		"region count": "AB\nAB\nCC\n",
		"row length":   "AAB\nAB\nCCB\n",
		"disconnected": "ABA\nBBB\nCCC\n",
	} {
		if _, err := Parse(strings.NewReader(text)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Parse(failingReader{}); err == nil || err.Error() != "boom" {
		t.Errorf("reader error: got %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParse did not panic")
		}
	}()
	MustParse("AB\n")
}

func TestValidate(t *testing.T) {
	big := make([][]int, MaxN+1)
	for r := range big {
		big[r] = make([]int, MaxN+1)
	}
	for name, regions := range map[string][][]int{
		"empty":        nil,
		"too large":    big,
		"out of range": {{0, 2}, {1, 1}},
		"negative":     {{0, -1}, {1, 1}},
		"missing":      {{0, 0}, {0, 0}},
	} {
		if _, err := New(regions); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestClone(t *testing.T) {
	p := MustParse("AAB\nACB\nCCB\n")
	q := p.Clone()
	q.Regions[0][0] = 2
	if p.Regions[0][0] != 0 {
		t.Fatal("Clone shares the region rows")
	}
	if q.N != p.N || q.String() == p.String() {
		t.Fatalf("clone %q vs original %q", q.String(), p.String())
	}
}

func TestUnits(t *testing.T) {
	p := MustParse("AAB\nACB\nCCB\n")
	units := p.Units()
	if len(units) != 9 {
		t.Fatalf("got %d units, want 9", len(units))
	}
	want := []string{"row 0", "row 1", "row 2", "col 0", "col 1", "col 2", "region A", "region B", "region C"}
	for i, u := range units {
		if u.String() != want[i] {
			t.Errorf("unit %d = %q, want %q", i, u, want[i])
		}
		for _, q := range u.Cells {
			if got := p.UnitOf(u.Kind, q); got != u.Index {
				t.Errorf("UnitOf(%v, %v) = %d, want %d", u.Kind, q, got, u.Index)
			}
		}
	}
}
