package concviz

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Layout of the sequence diagram.
const (
	sqCol    = 92.0 // column width
	sqGutter = 40.0 // step numbers on the left
	sqTop    = 50.0 // below the title
	sqHead   = 50.0 // column heads
	sqRow    = 26.0
	sqGap    = 10.0 // between steps
	sqFoot   = 44.0 // legend
	sqWrap   = 14   // characters per line of a column head
)

const seqStyle = `svg{--bg:#ffffff;--fg:#1f2328;--mut:#59636e;--ln:#c4ccd5;--band:#f6f8fa;--run:#1a7f37;--runbg:#dafbe1;--blk:#bc4c00;--blkbg:#fff1e5;--ch:#0969da;--chbg:#ddf4ff;--tok:#8250df;--cls:#cf222e;--box:#f6f8fa}
@media (prefers-color-scheme:dark){svg{--bg:#0d1117;--fg:#e6edf3;--mut:#9198a1;--ln:#3d444d;--band:#151b23;--run:#3fb950;--runbg:#12261e;--blk:#f0883e;--blkbg:#2d1b0e;--ch:#4493f8;--chbg:#0c2d6b;--tok:#a371f7;--cls:#f85149;--box:#151b23}}
text{font:11px system-ui,-apple-system,"Segoe UI",Helvetica,Arial,sans-serif;fill:var(--fg)}
.m{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:10.5px}
.bg{fill:var(--bg)}.ti{font-size:15px;font-weight:600}.mu{fill:var(--mut)}.band{fill:var(--band)}.sn{fill:var(--mut);text-anchor:middle;font-size:12px}
.h{text-anchor:middle;font-weight:600}.hg{fill:var(--runbg);stroke:var(--run)}.ho{fill:var(--bg);stroke:var(--mut);stroke-dasharray:4 3}.hc{fill:var(--chbg);stroke:var(--ch)}.hb{fill:var(--box);stroke:var(--ln)}
.lr{stroke:var(--run);stroke-width:2.5}.lb{stroke:var(--blk);stroke-width:4;stroke-dasharray:5 3}.lo{stroke:var(--mut);stroke-width:1.5;stroke-dasharray:4 3}
.lc{stroke:var(--ch);stroke-width:1.5}.lx{stroke:var(--cls);stroke-width:2}.lk{stroke:var(--ln);stroke-width:1.5;stroke-dasharray:2 3}
.end{stroke:var(--fg);stroke-width:2.5}
.av{stroke:var(--tok);stroke-width:1.8;fill:none}.ac{stroke:var(--cls);stroke-width:1.8;fill:none}.acd{stroke:var(--cls);stroke-width:1.5;fill:none;stroke-dasharray:4 3}.as{stroke:var(--mut);stroke-width:1.5;fill:none;stroke-dasharray:4 3}
.buf{fill:var(--tok);opacity:.35}.held{fill:var(--mut);opacity:.35}.dot{fill:var(--tok)}
.lab{text-anchor:middle;paint-order:stroke;stroke:var(--bg);stroke-width:3px}.tv{fill:var(--tok)}.tc{fill:var(--cls)}.ts{fill:var(--mut)}
.bl{fill:var(--blk);paint-order:stroke;stroke:var(--bg);stroke-width:3px}.no{fill:var(--mut);font-style:italic;paint-order:stroke;stroke:var(--bg);stroke-width:3px}
.k1{fill:var(--blkbg);stroke:var(--blk);stroke-width:1.5}.k0{fill:var(--runbg);stroke:var(--run);stroke-width:1.5}.kn{font-weight:700;text-anchor:middle;font-size:12px}
.mv{fill:var(--tok)}.mc{fill:var(--cls)}.ms{fill:var(--mut)}
`

// Sequence draws the story as a static sequence diagram: a column per
// goroutine, channel and box, time downwards, a band per step.
func (s *Story) Sequence() []byte {
	// Columns, ordered as in the scene: left to right, then top to bottom.
	var cols []node
	for _, g := range s.gs {
		cols = append(cols, g)
	}
	// Channels no event touches are left out (they show in what a blocked
	// goroutine waits on).
	for _, c := range s.cs {
		if slices.ContainsFunc(s.rows, func(r row) bool {
			e := r.ev
			return e != nil && (e.from == node(c) || e.to == node(c) || e.via == node(c))
		}) {
			cols = append(cols, c)
		}
	}
	for _, r := range s.rows {
		if e := r.ev; e != nil {
			for _, nd := range []node{e.from, e.to} {
				if b, ok := nd.(*Box); ok && !slices.Contains(cols, node(b)) {
					cols = append(cols, b)
				}
			}
		}
	}
	slices.SortStableFunc(cols, func(a, b node) int {
		pa, pb := a.bounds().center(), b.bounds().center()
		return cmp.Or(cmp.Compare(pa.X, pb.X), cmp.Compare(pa.Y, pb.Y))
	})
	col := map[node]int{}
	for i, c := range cols {
		col[c] = i
	}
	x := func(nd node) float64 { return sqGutter + sqCol*(float64(col[nd])+0.5) }

	// Rows: y of each, and the band of each step.
	ys := make([]float64, len(s.rows))
	type band struct {
		step     int
		top, bot float64
	}
	var bands []band
	y := sqTop + sqHead + 8
	for i, r := range s.rows {
		if i == 0 || r.step != s.rows[i-1].step {
			if i > 0 {
				bands[len(bands)-1].bot = y
				y += sqGap
			}
			bands = append(bands, band{step: r.step, top: y})
		}
		ys[i] = y + sqRow/2
		y += sqRow
	}
	bands[len(bands)-1].bot = y
	bottom := y + 6
	w := max(sqGutter+sqCol*float64(len(cols))+12, 32+float64(len([]rune(s.Title)))*8.6)
	legend, legendH := s.seqLegend(bottom+28, w)
	h := bottom + sqFoot + legendH

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">`,
		num(w), num(h), num(w), num(h), esc(s.Title))
	p("\n<title>%s</title>\n<style>\n%s</style>\n", esc(s.Title), seqStyle)
	for _, k := range []string{"v", "c", "s"} {
		p(`<marker id="a%s" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path class="m%s" d="M0 0L10 5L0 10z"/></marker>`, k, k)
	}
	p("\n")
	p(`<rect class="bg" width="%s" height="%s"/>`, num(w), num(h))
	p(`<text class="ti" x="16" y="26">%s</text>`, esc(s.Title))
	p(`<text class="m mu" x="16" y="42">%s · one possible run, time downwards</text>`, esc(s.Source))
	p("\n")
	for i, bd := range bands {
		if i%2 == 0 {
			p(`<rect class="band" x="4" y="%s" width="%s" height="%s" rx="4"/>`, num(bd.top-2), num(w-8), num(bd.bot-bd.top+4))
		}
		p(`<text class="sn m" x="%s" y="%s">%d</text>`, num(sqGutter/2), num(bd.top+14), bd.step)
	}
	p("\n")

	// Column heads.
	for _, nd := range cols {
		cx := x(nd)
		cls, name, sub := "hb", nd.label(), ""
		switch nd := nd.(type) {
		case *G:
			cls = "hg"
			if nd.outside {
				cls = "ho"
			}
		case *C:
			cls = "hc"
			switch {
			case nd.outside:
				sub = "outside"
			case nd.slots == 0:
				sub = "unbuffered"
			default:
				sub = fmt.Sprintf("cap %d", nd.slots)
			}
		}
		lines := wrapHead(name)
		if sub != "" {
			lines = append(lines, sub)
		}
		top := sqTop
		p(`<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="6"/>`, cls, num(cx-sqCol/2+4), num(top), num(sqCol-8), num(sqHead-6))
		ly := top + (sqHead-6)/2 - float64(len(lines)-1)*6.5 + 4
		for k, l := range lines {
			c := "h"
			if sub != "" && k == len(lines)-1 {
				c = "h mu"
			}
			if _, ok := nd.(*C); ok && k < len(lines)-1 {
				c = "h m"
			}
			p(`<text class="%s" x="%s" y="%s">%s</text>`, c, num(cx), num(ly+float64(k)*13), esc(l))
		}
	}
	p("\n")

	// Lifelines of channels and boxes: closed channels turn red.
	top := sqTop + sqHead
	for _, nd := range cols {
		cx := x(nd)
		switch nd.(type) {
		case *C:
			closeY := bottom
			for i, r := range s.rows {
				if e := r.ev; e != nil && e.kind == evClose && e.to == nd {
					closeY = ys[i]
				}
			}
			p(`<line class="lc" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(top), num(cx), num(closeY))
			if closeY < bottom {
				p(`<line class="lx" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(closeY), num(cx), num(bottom))
			}
		case *Box:
			p(`<line class="lk" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(top), num(cx), num(bottom))
		}
	}
	// Free labels (what a blocked goroutine waits on, notes) go where they
	// overlap no other label: right of the lifeline, else left, else lower.
	type area struct{ x0, y0, x1, y1 float64 }
	var taken []area
	free := func(r area) bool {
		if r.x0 < sqGutter || r.x1 > w-4 {
			return false
		}
		const gap = 6 // between labels side by side
		for _, t := range taken {
			if r.x0 < t.x1+gap && t.x0 < r.x1+gap && r.y0 < t.y1+3 && t.y0 < r.y1+3 {
				return false
			}
		}
		return true
	}
	var labels strings.Builder
	// place puts a label of one or more lines (separated by \n).
	place := func(cls string, cx, y float64, text string, charW float64) {
		lines := strings.Split(text, "\n")
		tw := 0.0
		for _, l := range lines {
			tw = max(tw, float64(len([]rune(l)))*charW)
		}
		th := float64(len(lines)-1) * 12
		x0, anchor, y0 := cx+6, "start", y
		found := false
		for k := 0; k < 6 && !found; k++ {
			yy := y + float64(k)*12
			if r := (area{cx + 6, yy - 9, cx + 6 + tw, yy + 3 + th}); free(r) {
				x0, anchor, y0, found = cx+6, "start", yy, true
				taken = append(taken, r)
			} else if r := (area{cx - 6 - tw, yy - 9, cx - 6, yy + 3 + th}); free(r) {
				x0, anchor, y0, found = cx-6, "end", yy, true
				taken = append(taken, r)
			}
		}
		fmt.Fprintf(&labels, `<text class="%s" x="%s" y="%s" style="text-anchor:%s">`, cls, num(x0), num(y0), anchor)
		for i, l := range lines {
			if i == 0 {
				fmt.Fprintf(&labels, `<tspan>%s</tspan>`, esc(l))
			} else {
				fmt.Fprintf(&labels, `<tspan x="%s" dy="12">%s</tspan>`, num(x0), esc(l))
			}
		}
		labels.WriteString(`</text>`)
	}

	// Events, first: their labels take their places before the free ones.
	var evs strings.Builder
	q := func(format string, a ...any) { fmt.Fprintf(&evs, format, a...) }
	label := func(cls string, mid, y float64, text string) {
		tw := float64(len([]rune(text))) * 6.3
		taken = append(taken, area{mid - tw/2, y - 10, mid + tw/2, y + 2})
		q(`<text class="lab m %s" x="%s" y="%s">%s</text>`, cls, num(mid), num(y), esc(text))
	}
	arrow := func(cls, mk string, from, to node, y float64, text, lcls string) {
		x1, x2 := x(from), x(to)
		d := 1.0
		if x2 < x1 {
			d = -1
		}
		q(`<line class="%s" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#a%s)"/>`, cls, num(x1+3*d), num(y), num(x2-4*d), num(y), mk)
		if text != "" {
			label(lcls, (x1+x2)/2, y-4, text)
		}
	}
	var notes []func()
	for i, r := range s.rows {
		e := r.ev
		if e == nil {
			continue
		}
		y := ys[i]
		switch e.kind {
		case evSpawn:
			if e.from != nil {
				arrow("as", "s", e.from, e.to, y, e.label, "ts")
			} else {
				cx := x(e.to)
				notes = append(notes, func() { place("no", cx, y+4, "starts", 5.8) })
			}
		case evSend, evGive, evRecv, evTake:
			arrow("av", "v", e.from, e.to, y, e.label, "tv")
		case evHandoff:
			x1, xc, x2 := x(e.from), x(e.via), x(e.to)
			d := 1.0
			if x2 < xc {
				d = -1
			}
			q(`<path class="av" d="M%s %sH%sV%sH%s" marker-end="url(#av)"/>`, num(x1), num(y-4), num(xc), num(y+4), num(x2-4*d))
			q(`<circle class="dot" cx="%s" cy="%s" r="3"/>`, num(xc), num(y))
			label("tv", (x1+xc)/2, y-8, e.label)
		case evClose:
			arrow("ac", "c", e.from, e.to, y, "close", "tc")
		case evClosed:
			x1, x2 := x(e.from), x(e.to)
			d := 1.0
			if x2 < x1 {
				d = -1
			}
			q(`<line class="acd" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#ac)"/>`, num(x1), num(y), num(x2-4*d), num(y))
			label("tc", (x1+x2)/2, y-4, "closed")
		case evNote:
			cx, text := x(e.from), e.label
			notes = append(notes, func() { place("no", cx, y+4, text, 5.8) })
		case evExit:
			if e.wg != nil { // ends at the counter, not under it
				x1, x2 := x(e.from), x(e.to)
				d := 1.0
				if x2 < x1 {
					d = -1
				}
				q(`<line class="as" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#as)"/>`, num(x1+3*d), num(y), num(x2-17*d), num(y))
				label("ts", (x1+x2)/2, y-4, e.label)
			} else if e.to != nil {
				arrow("as", "s", e.from, e.to, y, e.label, "ts")
			}
		case evWake, evLock, evUnlock:
			arrow("as", "s", e.from, e.to, y, e.label, "ts")
		}
	}
	// WaitGroup counters, where they change: orange while Wait would block.
	var counters strings.Builder
	for i, r := range s.rows {
		if e := r.ev; e != nil && e.wg != nil {
			cx, y, cls := x(e.wg.box), ys[i], "k1"
			if e.n == 0 {
				cls = "k0"
			}
			taken = append(taken, area{cx - 13, y - 9, cx + 13, y + 9})
			fmt.Fprintf(&counters, `<rect class="%s" x="%s" y="%s" width="26" height="18" rx="9"/><text class="kn" x="%s" y="%s">%d</text>`,
				cls, num(cx-13), num(y-9), num(cx), num(y+4.5), e.n)
		}
	}
	for _, f := range notes {
		f()
	}

	// Lifelines of goroutines: a segment per stretch of the same state.
	for gi, g := range s.gs {
		cx := x(g)
		type seg struct {
			st       state
			status   string
			from, to float64
		}
		var segs []seg
		add := func(f gFrame, from, to float64) {
			if k := len(segs); k > 0 && segs[k-1].st == f.st && segs[k-1].status == f.status && segs[k-1].to == from {
				segs[k-1].to = to
				return
			}
			segs = append(segs, seg{f.st, f.status, from, to})
		}
		add(s.frames[0].gs[gi], top, ys[0])
		for i, r := range s.rows {
			to := bottom
			if i+1 < len(s.rows) {
				to = ys[i+1]
			}
			add(r.after[gi], ys[i], to)
		}
		prev := hidden
		for _, sg := range segs {
			switch {
			case sg.st == running && g.outside:
				p(`<line class="lo" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(sg.from), num(cx), num(sg.to))
			case sg.st == running:
				p(`<line class="lr" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(sg.from), num(cx), num(sg.to))
			case sg.st == blocked:
				p(`<line class="lb" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx), num(sg.from), num(cx), num(sg.to))
				place("bl m", cx, sg.from+13, waitsOn(sg.status), 6.3)
			case sg.st == exited && prev != exited:
				p(`<line class="end" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(cx-8), num(sg.from), num(cx+8), num(sg.from))
			}
			prev = sg.st
		}
	}
	p("\n")

	// Values waiting in a buffer: a bar on the channel from send to receive.
	for i, r := range s.rows {
		e := r.ev
		if e == nil || e.kind != evSend {
			continue
		}
		end := bottom
		for j := i + 1; j < len(s.rows); j++ {
			if f := s.rows[j].ev; f != nil && f.kind == evRecv && f.tok == e.tok {
				end = ys[j]
				break
			}
		}
		cx := x(e.to)
		p(`<rect class="buf" x="%s" y="%s" width="10" height="%s" rx="5"/>`, num(cx-5), num(ys[i]-5), num(end-ys[i]+10))
	}
	b.WriteString(evs.String())
	// A held mutex: a bar on its column from Lock (or Lock returns) to Unlock.
	for i, r := range s.rows {
		e := r.ev
		var m node
		switch {
		case e == nil:
		case e.kind == evLock:
			m = e.to
		case e.kind == evWake && e.label == "Lock returns":
			m = e.from
		}
		if m == nil {
			continue
		}
		end := bottom
		for j := i + 1; j < len(s.rows); j++ {
			if f := s.rows[j].ev; f != nil && f.kind == evUnlock && f.to == m {
				end = ys[j]
				break
			}
		}
		cx := x(m)
		p(`<rect class="held" x="%s" y="%s" width="10" height="%s" rx="2"/>`, num(cx-5), num(ys[i]), num(end-ys[i]))
	}
	b.WriteString(counters.String())
	b.WriteString(labels.String())
	p("\n")
	p("%s", legend)
	p("\n</svg>\n")
	return []byte(b.String())
}

// waitsOn shortens a blocked status to what the goroutine waits on; a long
// select keeps its two lines.
func waitsOn(status string) string {
	st := strings.TrimPrefix(status, "blocked: ")
	if rest, ok := strings.CutPrefix(st, "blocked in select:\n"); ok {
		st = "select " + rest
	}
	return st
}

// wrapHead breaks a column name into lines; long code names break after a dot.
func wrapHead(name string) []string {
	var lines []string
	for _, l := range wrap(name, sqWrap) {
		for len([]rune(l)) > sqWrap {
			cut := strings.LastIndex(l[:sqWrap], ".") + 1 // the dot stays on the first line
			if cut <= 1 {
				cut = sqWrap
			}
			lines = append(lines, l[:cut])
			l = l[cut:]
		}
		lines = append(lines, l)
	}
	return lines
}

// seqLegend draws the legend from y on, in lines no wider than width, and
// returns it with the height it adds beyond its first line.
func (s *Story) seqLegend(y, width float64) (string, float64) {
	var b strings.Builder
	x, y0 := 16.0, y
	item := func(label string, shape func() string) {
		w := 30 + float64(len([]rune(label)))*5.6
		if x > 16 && x+w > width-8 {
			x, y = 16, y+20
		}
		b.WriteString(shape())
		fmt.Fprintf(&b, `<text class="mu" x="%s" y="%s">%s</text>`, num(x+30), num(y+4), label)
		x += w + 18
	}
	line := func(cls, mk string) func() string {
		return func() string {
			m := ""
			if mk != "" {
				m = ` marker-end="url(#a` + mk + `)"`
			}
			return fmt.Sprintf(`<line class="%s" x1="%s" y1="%s" x2="%s" y2="%s"%s/>`, cls, num(x), num(y), num(x+24), num(y), m)
		}
	}
	item("running", line("lr", ""))
	item("blocked, waiting on …", line("lb", ""))
	item("returns", func() string {
		return fmt.Sprintf(`<line class="end" x1="%s" y1="%s" x2="%s" y2="%s"/>`, num(x+4), num(y), num(x+20), num(y))
	})
	item("value", line("av", "v"))
	item("in a buffer", func() string {
		return fmt.Sprintf(`<rect class="buf" x="%s" y="%s" width="24" height="10" rx="5"/>`, num(x), num(y-5))
	})
	item("close", line("ac", "c"))
	if slices.ContainsFunc(s.rows, func(r row) bool { return r.ev != nil && r.ev.kind == evLock }) {
		item("mutex held", func() string {
			return fmt.Sprintf(`<rect class="held" x="%s" y="%s" width="24" height="10" rx="2"/>`, num(x), num(y-5))
		})
	}
	arrows := "go, done, Wait returns"
	if slices.ContainsFunc(s.rows, func(r row) bool { return r.ev != nil && r.ev.kind == evLock }) {
		arrows = "go, done, Lock, Unlock, Lock returns"
	}
	item(arrows, line("as", "s"))
	if slices.ContainsFunc(s.boxes, func(b *Box) bool { return b.wg }) {
		item("WaitGroup counter", func() string {
			return fmt.Sprintf(`<rect class="k1" x="%s" y="%s" width="24" height="16" rx="8"/><text class="kn" x="%s" y="%s">1</text>`,
				num(x), num(y-8), num(x+12), num(y+4))
		})
	}
	return b.String(), y - y0
}
