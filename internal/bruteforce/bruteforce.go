// Package bruteforce is an exhaustive solver tuned for speed: it checks
// uniqueness in the generator and is the test oracle for the observer-based
// solver.
//
// It branches row by row (row 0 first, columns low to high), so solutions come
// out in lexicographic order, exactly as plain row-by-row backtracking would
// find them. Everything else is sound pruning and forced moves, which never
// change the set of solutions below a node, so Solve(p, limit) always returns
// the lexicographically first limit solutions. The generator depends on this:
// the alternatives it sees decide which puzzle a seed gives.
//
// State is one uint64 column mask per row (n <= 64; puzzle.MaxN is 52).
package bruteforce

import (
	"math/bits"

	"starbattle/internal/puzzle"
)

// Solve returns up to limit solutions. Each solution gives the star column per row.
func Solve(p *puzzle.Puzzle, limit int) [][]int {
	out, _ := solve(p, limit)
	return out
}

// CountSolutions returns the number of solutions, capped at limit.
func CountSolutions(p *puzzle.Puzzle, limit int) int {
	return len(Solve(p, limit))
}

type state struct{ placed, usedCol, usedReg uint64 }

type solver struct {
	n            int
	full         uint64 // all columns (and all rows, all regions: same n)
	cellReg      []uint8
	regRow       []uint64 // [reg*n+row]: the region's columns in that row
	regLo, regHi []int
	segOff       []int // row i's (region, mask) pairs are segReg/segMask[segOff[i]:segOff[i+1]]
	segReg       []uint8
	segMask      []uint64
	cnt          []int // scratch for propagate
	lastRow      []int
	lastBits     []uint64
	colsOf       []uint64 // scratch: columns of each region's remaining cells
	rowsOf       []uint64 // scratch: rows of each region's remaining cells
	regsOf       []uint64 // scratch: regions of each row's remaining cells
	nodes        uint64
}

// solve also returns the number of placements tried (for tests).
func solve(p *puzzle.Puzzle, limit int) ([][]int, uint64) {
	n := p.N
	if n > 64 {
		panic("bruteforce: n > 64")
	}
	sv := &solver{
		n:        n,
		full:     ^uint64(0) >> (64 - n),
		cellReg:  make([]uint8, n*n),
		regRow:   make([]uint64, n*n),
		regLo:    make([]int, n),
		regHi:    make([]int, n),
		segOff:   make([]int, n+1),
		cnt:      make([]int, n),
		lastRow:  make([]int, n),
		lastBits: make([]uint64, n),
		colsOf:   make([]uint64, n),
		rowsOf:   make([]uint64, n),
		regsOf:   make([]uint64, n),
	}
	for g := range n {
		sv.regLo[g], sv.regHi[g] = n, -1
	}
	for r := range n {
		for c := range n {
			g := p.Regions[r][c]
			sv.cellReg[r*n+c] = uint8(g)
			sv.regRow[g*n+r] |= 1 << c
			sv.regLo[g] = min(sv.regLo[g], r)
			sv.regHi[g] = max(sv.regHi[g], r)
		}
	}
	for r := range n {
		sv.segOff[r] = len(sv.segReg)
		for g := range n {
			if m := sv.regRow[g*n+r]; m != 0 {
				sv.segReg = append(sv.segReg, uint8(g))
				sv.segMask = append(sv.segMask, m)
			}
		}
	}
	sv.segOff[n] = len(sv.segReg)

	// One level per branching row; each level places at least one star.
	sts := make([]state, n+1)
	avail := make([]uint64, (n+1)*n)
	cols := make([]uint8, (n+1)*n)
	brRow := make([]int, n+1)
	brBits := make([]uint64, n+1)

	var out [][]int
	record := func(cl []uint8) bool {
		s := make([]int, n)
		for i, c := range cl {
			s[i] = int(c)
		}
		out = append(out, s)
		return len(out) >= limit
	}

	for i := range n {
		avail[i] = sv.full
	}
	if !sv.propagate(&sts[0], avail[:n], cols[:n]) {
		return nil, sv.nodes
	}
	if sts[0].placed == sv.full {
		record(cols[:n])
		return out, sv.nodes
	}
	r0 := bits.TrailingZeros64(^sts[0].placed)
	brRow[0], brBits[0] = r0, avail[r0]

	d := 0
	for d >= 0 {
		b := brBits[d]
		if b == 0 {
			d--
			continue
		}
		brBits[d] = b & (b - 1)
		c := bits.TrailingZeros64(b)
		r := brRow[d]
		sv.nodes++
		st := &sts[d+1]
		*st = sts[d]
		a := avail[(d+1)*n : (d+2)*n]
		copy(a, avail[d*n:(d+1)*n])
		cl := cols[(d+1)*n : (d+2)*n]
		copy(cl, cols[d*n:(d+1)*n])
		sv.place(st, a, cl, r, c)
		if !sv.propagate(st, a, cl) {
			continue
		}
		if st.placed == sv.full {
			if record(cl) {
				return out, sv.nodes
			}
			continue
		}
		d++
		nr := bits.TrailingZeros64(^st.placed)
		brRow[d], brBits[d] = nr, a[nr]
	}
	return out, sv.nodes
}

func (sv *solver) place(st *state, a []uint64, cl []uint8, r, c int) {
	n := sv.n
	b := uint64(1) << c
	g := sv.cellReg[r*n+c]
	st.placed |= 1 << r
	st.usedCol |= b
	st.usedReg |= 1 << g
	cl[r] = uint8(c)
	for i := range a {
		a[i] &^= b
	}
	a[r] = 0
	nb := b | b<<1 | b>>1
	if r > 0 {
		a[r-1] &^= nb
	}
	if r+1 < n {
		a[r+1] &^= nb
	}
	rr := sv.regRow[int(g)*n:]
	for i, hi := sv.regLo[g], sv.regHi[g]; i <= hi; i++ {
		a[i] &^= rr[i]
	}
}

// propagate places forced stars until none are left. It returns false if some
// row, column or region can no longer get a star.
func (sv *solver) propagate(st *state, a []uint64, cl []uint8) bool {
outer:
	for {
		rem := sv.full &^ st.placed
		if rem == 0 {
			return true
		}
		var once, twice uint64
		for m := rem; m != 0; m &= m - 1 {
			i := bits.TrailingZeros64(m)
			x := a[i]
			if x == 0 {
				return false
			}
			if x&(x-1) == 0 {
				sv.place(st, a, cl, i, bits.TrailingZeros64(x))
				continue outer
			}
			twice |= once & x
			once |= x
		}
		need := sv.full &^ st.usedCol
		if once&need != need {
			return false
		}
		if single := need &^ twice; single != 0 {
			c := bits.TrailingZeros64(single)
			for m := rem; m != 0; m &= m - 1 {
				i := bits.TrailingZeros64(m)
				if a[i]>>c&1 != 0 {
					sv.place(st, a, cl, i, c)
					continue outer
				}
			}
		}

		n := sv.n
		cnt := sv.cnt
		free := sv.full &^ st.usedReg
		for m := free; m != 0; m &= m - 1 {
			g := bits.TrailingZeros64(m)
			cnt[g] = 0
			sv.rowsOf[g] = 0
			sv.colsOf[g] = 0
		}
		for m := rem; m != 0; m &= m - 1 {
			i := bits.TrailingZeros64(m)
			x := a[i]
			var rs uint64
			for k, e := sv.segOff[i], sv.segOff[i+1]; k < e; k++ {
				if y := x & sv.segMask[k]; y != 0 {
					g := sv.segReg[k]
					cnt[g] += bits.OnesCount64(y)
					sv.lastRow[g] = i
					sv.lastBits[g] = y
					sv.rowsOf[g] |= 1 << i
					sv.colsOf[g] |= y
					rs |= 1 << g
				}
			}
			sv.regsOf[i] = rs
		}
		for m := free; m != 0; m &= m - 1 {
			g := bits.TrailingZeros64(m)
			switch cnt[g] {
			case 0:
				return false
			case 1:
				sv.place(st, a, cl, sv.lastRow[g], bits.TrailingZeros64(sv.lastBits[g]))
				continue outer
			}
		}

		// Hall: k regions whose cells lie in k rows (columns) own those rows
		// (columns); k rows whose cells lie in k regions own those regions.
		// Seeds are single regions or rows; more than k is a contradiction.
		changed := false
		left := bits.OnesCount64(rem)
		for m := free; m != 0; m &= m - 1 {
			u := sv.rowsOf[bits.TrailingZeros64(m)]
			if bits.OnesCount64(u) >= left {
				continue
			}
			var s uint64
			for mm := free; mm != 0; mm &= mm - 1 {
				h := bits.TrailingZeros64(mm)
				if sv.rowsOf[h]&^u == 0 {
					s |= 1 << h
				}
			}
			k, ku := bits.OnesCount64(s), bits.OnesCount64(u)
			if k > ku {
				return false
			}
			if k == ku {
				for mm := u; mm != 0; mm &= mm - 1 {
					i := bits.TrailingZeros64(mm)
					var keep uint64
					for ss := s; ss != 0; ss &= ss - 1 {
						keep |= sv.regRow[bits.TrailingZeros64(ss)*n+i]
					}
					if a[i]&^keep != 0 {
						a[i] &= keep
						changed = true
					}
				}
			}
		}
		for m := free; m != 0; m &= m - 1 {
			u := sv.colsOf[bits.TrailingZeros64(m)]
			if bits.OnesCount64(u) >= left {
				continue
			}
			var s uint64
			for mm := free; mm != 0; mm &= mm - 1 {
				h := bits.TrailingZeros64(mm)
				if sv.colsOf[h]&^u == 0 {
					s |= 1 << h
				}
			}
			k, ku := bits.OnesCount64(s), bits.OnesCount64(u)
			if k > ku {
				return false
			}
			if k == ku {
				for mm := rem; mm != 0; mm &= mm - 1 {
					i := bits.TrailingZeros64(mm)
					var keep uint64
					for ss := s; ss != 0; ss &= ss - 1 {
						keep |= sv.regRow[bits.TrailingZeros64(ss)*n+i]
					}
					if drop := a[i] & u &^ keep; drop != 0 {
						a[i] &^= drop
						changed = true
					}
				}
			}
		}
		for m := rem; m != 0; m &= m - 1 {
			u := sv.regsOf[bits.TrailingZeros64(m)]
			if bits.OnesCount64(u) >= left {
				continue
			}
			var s uint64
			for mm := rem; mm != 0; mm &= mm - 1 {
				j := bits.TrailingZeros64(mm)
				if sv.regsOf[j]&^u == 0 {
					s |= 1 << j
				}
			}
			k, ku := bits.OnesCount64(s), bits.OnesCount64(u)
			if k > ku {
				return false
			}
			if k == ku {
				for mm := rem &^ s; mm != 0; mm &= mm - 1 {
					j := bits.TrailingZeros64(mm)
					for gg := u & sv.regsOf[j]; gg != 0; gg &= gg - 1 {
						if rr := sv.regRow[bits.TrailingZeros64(gg)*n+j]; a[j]&rr != 0 {
							a[j] &^= rr
							changed = true
						}
					}
				}
			}
		}
		if !changed {
			return true
		}
	}
}
