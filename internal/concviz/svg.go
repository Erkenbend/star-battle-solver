package concviz

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Timing, in seconds.
const (
	moveTime  = 1.1  // a step in which values move
	stillTime = 0.35 // a step in which only states change
	readBase  = 1.3  // reading time of every caption …
	readChar  = 0.03 // … plus this per character
	lastHold  = 3.0  // extra time on the last step before the loop restarts
	fade      = 0.25
)

// Text sizes, to wrap captions and to size values.
const (
	capChar   = 7.3 // average width of a caption character (14px sans)
	monoChar  = 6.6 // width of a monospace character (11px)
	capLines  = 3
	capHeight = 112.0 // caption and legend at the bottom
)

const style = `svg{--bg:#ffffff;--fg:#1f2328;--mut:#59636e;--ln:#c4ccd5;--run:#1a7f37;--runbg:#dafbe1;--blk:#bc4c00;--blkbg:#fff1e5;--ext:#818b98;--extbg:#f6f8fa;--ch:#0969da;--chbg:#ddf4ff;--tok:#8250df;--tokfg:#ffffff;--cls:#cf222e;--box:#f6f8fa}
@media (prefers-color-scheme:dark){svg{--bg:#0d1117;--fg:#e6edf3;--mut:#9198a1;--ln:#3d444d;--run:#3fb950;--runbg:#12261e;--blk:#f0883e;--blkbg:#2d1b0e;--ext:#656c76;--extbg:#151b23;--ch:#4493f8;--chbg:#0c2d6b;--tok:#a371f7;--tokfg:#ffffff;--cls:#f85149;--box:#151b23}}
text{font:13px system-ui,-apple-system,"Segoe UI",Helvetica,Arial,sans-serif;fill:var(--fg)}
.m{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:11px}
.bg{fill:var(--bg)}.ti{font-size:16px;font-weight:600}.so,.mu{fill:var(--mut)}
.fl{stroke:var(--ln);stroke-width:2;fill:none}.fa{fill:var(--ln)}
.wt{stroke:var(--blk);stroke-width:1.5;stroke-dasharray:5 4;fill:none}.wa{fill:var(--blk)}
.g{stroke-width:1.5}.run{fill:var(--runbg);stroke:var(--run)}.blk{fill:var(--blkbg);stroke:var(--blk)}.ext{fill:var(--extbg);stroke:var(--ext)}.out{fill:var(--bg);stroke:var(--mut);stroke-dasharray:4 3}
.gn{font-weight:600;text-anchor:middle}.gf{fill:var(--mut);text-anchor:middle}.gs{text-anchor:middle}
.s-run{fill:var(--run)}.s-blk{fill:var(--blk)}.s-ext{fill:var(--ext)}.s-out{fill:var(--mut)}
.ch{fill:var(--chbg);stroke:var(--ch);stroke-width:1.5}.sl{fill:none;stroke:var(--ch);stroke-dasharray:3 3;opacity:.6}.cv{fill:none;stroke:var(--ch);stroke-width:2}
.cn{font-weight:700;font-size:12px;text-anchor:middle}.cm{fill:var(--mut);text-anchor:middle;font-size:10.5px}
.cx{fill:var(--bg);stroke:var(--cls);stroke-width:2}.ct{fill:var(--cls);font-weight:700;text-anchor:middle}
.bx{fill:var(--box);stroke:var(--ln);stroke-width:1.5}.bt{font-weight:600;font-size:12px}
.tk{fill:var(--tok)}.tl{fill:var(--tokfg);text-anchor:middle}
.sep{stroke:var(--ln)}.cp{font-size:14px}.cn2{fill:var(--mut);font-size:12px}
`

// timeline returns when each step starts (t0) and when its movement ends and
// the states change (t1), for steps 1 to n; t0[n+1] = t1[n+1] = total.
func (s *Story) timeline() (t0, t1 []float64, total float64) {
	n := len(s.frames) - 1
	t0 = make([]float64, n+2)
	t1 = make([]float64, n+2)
	t := 0.0
	for i := 1; i <= n; i++ {
		f := s.frames[i]
		m := stillTime
		if f.moves() {
			m = moveTime
		}
		hold := readBase + readChar*float64(len(f.caption))
		if i == n {
			hold += lastHold
		}
		t0[i], t1[i] = t, t+m
		t += m + hold
	}
	t0[n+1], t1[n+1] = t, t
	return t0, t1, t
}

// Scene draws the story as an animated SVG: the scene the player shows.
func (s *Story) Scene() []byte {
	if len(s.frames) < 2 {
		s.fail("no steps")
	}
	n := len(s.frames) - 1
	t0, t1, total := s.timeline()

	a := &anim{total: total}
	var out strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&out, format, args...) }

	// Static: background, title, flows, frames of channels and boxes.
	var static strings.Builder
	sw := func(format string, args ...any) { fmt.Fprintf(&static, format, args...) }
	sw(`<rect class="bg" width="%s" height="%s"/>`, num(s.W), num(s.H))
	sw(`<text class="ti" x="20" y="30">%s</text>`, esc(s.Title))
	sw(`<text class="so m" x="%s" y="30" text-anchor="end">%s</text>`, num(s.W-20), esc(s.Source))
	// A flow to or from a goroutine shows once the goroutine is started.
	flowGs := make([][]int, len(s.flows))
	for i, f := range s.flows {
		a1, b1 := f[0].bounds(), f[1].bounds()
		p := a1.border(b1.center())
		q := b1.border(a1.center())
		body := fmt.Sprintf(`<line class="fl" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#af)"/>`, num(p.X), num(p.Y), num(q.X), num(q.Y))
		for _, nd := range f {
			if g, ok := nd.(*G); ok {
				flowGs[i] = append(flowGs[i], slices.Index(s.gs, g))
			}
		}
		if flowGs[i] == nil {
			static.WriteString(body)
			continue
		}
		for k := 0; k <= n; k++ {
			shown := true
			for _, j := range flowGs[i] {
				shown = shown && s.frames[k].gs[j].st != hidden
			}
			if shown {
				a.show(0, body, t1[k], t1[k+1])
			}
		}
	}
	for _, c := range s.cs {
		r := c.bounds()
		sw(`<rect class="ch" x="%s" y="%s" width="%s" height="%s" rx="6"/>`, num(r.x), num(r.y), num(r.w), num(r.h))
		if c.slots == 0 {
			d := float64(c.dir)
			sw(`<path class="cv" d="M%s %sl%s 7l%s 7M%s %sl%s 7l%s 7"/>`,
				num(c.at.X-12*d), num(c.at.Y-7), num(8*d), num(-8*d), num(c.at.X), num(c.at.Y-7), num(8*d), num(-8*d))
		}
		for i := range c.slots {
			p := c.slotPos(i)
			sw(`<rect class="sl" x="%s" y="%s" width="%s" height="%s" rx="10"/>`, num(p.X-slotW/2+3), num(p.Y-tokH/2-1), num(slotW-6), num(tokH+2))
		}
		nx, ny, mx, my, anchor := c.at.X, r.y-6, c.at.X, r.y+r.h+13, "middle"
		switch c.side {
		case 'b':
			ny, my = r.y+r.h+14, r.y+r.h+27
		case 'l':
			nx, ny, mx, my, anchor = r.x-8, c.at.Y-3, r.x-8, c.at.Y+11, "end"
		case 'r':
			nx, ny, mx, my, anchor = r.x+r.w+8, c.at.Y-3, r.x+r.w+8, c.at.Y+11, "start"
		}
		sw(`<text class="cn m" x="%s" y="%s" style="text-anchor:%s">%s</text>`, num(nx), num(ny), anchor, esc(c.name))
		sw(`<text class="cm m" x="%s" y="%s" style="text-anchor:%s">%s</text>`, num(mx), num(my), anchor, esc(c.describe()))
	}
	for _, b := range s.boxes {
		r := b.r
		sw(`<rect class="bx" x="%s" y="%s" width="%s" height="%s" rx="6"/>`, num(r.x), num(r.y), num(r.w), num(r.h))
		sw(`<text class="bt" x="%s" y="%s">%s</text>`, num(r.x+8), num(r.y+16), esc(b.title))
	}

	// States: goroutines, closed channels, box contents, who waits on what.
	window := func(i int) (float64, float64) { return t1[i], t1[i+1] }
	for i := 0; i <= n; i++ {
		f := s.frames[i]
		from, to := window(i)
		for _, e := range f.waits {
			g := s.gs[e.g]
			p := g.bounds().border(e.to.center())
			q := e.to.border(g.at)
			// Beside, not on top of, a flow between the same two.
			d := math.Hypot(q.X-p.X, q.Y-p.Y)
			ox, oy := -(q.Y-p.Y)/d*5, (q.X-p.X)/d*5
			p, q = Pt{p.X + ox, p.Y + oy}, Pt{q.X + ox, q.Y + oy}
			a.show(1, fmt.Sprintf(`<line class="wt" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#aw)"/>`,
				num(p.X), num(p.Y), num(q.X), num(q.Y)), from, to)
		}
		for j, gf := range f.gs {
			if gf.st != hidden {
				a.show(2, s.gs[j].draw(gf), from, to)
			}
		}
		for j, closed := range f.closed {
			if closed {
				a.show(2, s.cs[j].drawClosed(), from, to)
			}
		}
		for j, lines := range f.boxes {
			r := s.boxes[j].r
			var b strings.Builder
			for k, l := range lines {
				fmt.Fprintf(&b, `<text class="m" x="%s" y="%s">%s</text>`, num(r.x+8), num(r.y+33+float64(k)*15), esc(l))
			}
			if b.Len() > 0 {
				a.show(2, b.String(), from, to)
			}
		}
	}
	// Captions.
	capY := s.H - capHeight + 26
	for i := 1; i <= n; i++ {
		lines := wrap(s.frames[i].caption, int((s.W-90)/capChar))
		if len(lines) > capLines {
			s.fail("caption of step %d needs more than %d lines: %q", i, capLines, s.frames[i].caption)
		}
		var b strings.Builder
		fmt.Fprintf(&b, `<text class="cn2" x="20" y="%s">%d/%d</text>`, num(capY), i, n)
		for k, l := range lines {
			fmt.Fprintf(&b, `<text class="cp" x="64" y="%s">%s</text>`, num(capY+float64(k)*19), esc(l))
		}
		a.show(4, b.String(), t0[i], t0[i+1])
	}
	// Values.
	for j, tk := range s.toks {
		a.token(tk, s.frames, j, t0, t1)
	}
	a.render()

	w(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">`,
		num(s.W), num(s.H), num(s.W), num(s.H), esc(s.Title))
	w("\n<title>%s</title>\n<style>\n%s", esc(s.Title), style)
	w(".v{opacity:0;animation:%ss linear infinite}.t{opacity:0;animation:%ss ease-in-out infinite}\n", num(total), num(total))
	out.WriteString(a.css.String())
	w("</style>\n")
	w(`<defs><marker id="af" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path class="fa" d="M0 0L10 5L0 10z"/></marker>`)
	w(`<marker id="aw" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto"><path class="wa" d="M0 0L10 5L0 10z"/></marker></defs>` + "\n")
	out.WriteString(static.String())
	out.WriteString("\n")
	for layer := range a.layers {
		for _, v := range a.layers[layer] {
			out.WriteString(v)
			out.WriteString("\n")
		}
	}
	// Caption area and legend.
	sepY := s.H - capHeight
	w(`<line class="sep" x1="20" y1="%s" x2="%s" y2="%s"/>`, num(sepY), num(s.W-20), num(sepY))
	out.WriteString(s.legend())
	out.WriteString("\n</svg>\n")
	return []byte(out.String())
}

func (g *G) draw(f gFrame) string {
	r := g.bounds()
	cls := map[state]string{running: "run", blocked: "blk", exited: "ext"}[f.st]
	if g.outside {
		cls = "out"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<rect class="g %s" x="%s" y="%s" width="%s" height="%s" rx="8"/>`, cls, num(r.x), num(r.y), num(r.w), num(r.h))
	fmt.Fprintf(&b, `<text class="gn" x="%s" y="%s">%s</text>`, num(g.at.X), num(g.at.Y-17), esc(g.name))
	if !g.outside {
		fmt.Fprintf(&b, `<text class="gf m" x="%s" y="%s">%s</text>`, num(g.at.X), num(g.at.Y-2), esc(g.fns[0]))
	}
	for k, l := range strings.Split(f.status, "\n") {
		if l != "" {
			fmt.Fprintf(&b, `<text class="gs m s-%s" x="%s" y="%s">%s</text>`, cls, num(g.at.X), num(g.at.Y+15+float64(k)*13), esc(l))
		}
	}
	return b.String()
}

func (c *C) drawClosed() string {
	r := c.bounds()
	return fmt.Sprintf(`<rect class="cx" x="%s" y="%s" width="%s" height="%s" rx="6"/><text class="ct" x="%s" y="%s">closed</text>`,
		num(r.x), num(r.y), num(r.w), num(r.h), num(c.at.X), num(c.at.Y+5))
}

func (s *Story) legend() string {
	y := s.H - 18
	x := 20.0
	var b strings.Builder
	item := func(shape string, w float64, label string) {
		b.WriteString(shape)
		fmt.Fprintf(&b, `<text class="mu" x="%s" y="%s" font-size="12">%s</text>`, num(x+w+6), num(y+4), label)
		x += w + 6 + float64(len(label))*6.6 + 22
	}
	box := func(cls string) string {
		return fmt.Sprintf(`<rect class="g %s" x="%s" y="%s" width="22" height="14" rx="3"/>`, cls, num(x), num(y-7))
	}
	item(box("run"), 22, "running")
	item(box("blk"), 22, "blocked")
	item(fmt.Sprintf(`<line class="wt" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#aw)"/>`, num(x), num(y), num(x+26), num(y)), 28, "waits on")
	item(box("ext"), 22, "returned")
	item(fmt.Sprintf(`<rect class="ch" x="%s" y="%s" width="30" height="14" rx="3"/>`, num(x), num(y-7)), 30, "channel")
	item(fmt.Sprintf(`<rect class="tk" x="%s" y="%s" width="26" height="14" rx="7"/>`, num(x), num(y-7)), 26, "value")
	item(fmt.Sprintf(`<line class="fl" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#af)"/>`, num(x), num(y), num(x+26), num(y)), 28, "data flow")
	return b.String()
}

// anim collects the parts that change: each is drawn once and shown in its
// time windows by an opacity animation of its own.
type anim struct {
	total  float64
	layers [5][]string
	idx    map[string]int // body → index into bodies
	bodies []variant
	tokens []*tokAnim
	css    strings.Builder
	n      int
}

type variant struct {
	layer int
	body  string
	wins  [][2]float64
}

func (a *anim) show(layer int, body string, from, to float64) {
	if a.idx == nil {
		a.idx = map[string]int{}
	}
	if to <= from {
		return
	}
	i, ok := a.idx[body]
	if !ok {
		i = len(a.bodies)
		a.idx[body] = i
		a.bodies = append(a.bodies, variant{layer: layer, body: body})
	}
	v := &a.bodies[i]
	if k := len(v.wins); k > 0 && from-v.wins[k-1][1] < 1e-9 {
		v.wins[k-1][1] = to
	} else {
		v.wins = append(v.wins, [2]float64{from, to})
	}
}

// render draws the variants and values into their layers, and their
// keyframes into css.
func (a *anim) render() {
	for _, v := range a.bodies {
		name := a.keys(v.wins)
		a.layers[v.layer] = append(a.layers[v.layer], fmt.Sprintf(`<g class="v" style="animation-name:%s">%s</g>`, name, v.body))
	}
	for _, tk := range a.tokens {
		a.layers[3] = append(a.layers[3], tk.markup(a))
	}
}

func (a *anim) pct(t float64) string {
	return strconv.FormatFloat(min(100, max(0, 100*t/a.total)), 'f', 3, 64)
}

// keys writes the keyframes that show a variant in wins and returns their name.
func (a *anim) keys(wins [][2]float64) string {
	type kf struct{ t, o float64 }
	var ks []kf
	add := func(t, o float64) {
		if k := len(ks); k > 0 && t <= ks[k-1].t {
			t = ks[k-1].t
			if t == ks[k-1].t {
				ks[k-1].o = o
				return
			}
		}
		ks = append(ks, kf{t, o})
	}
	if wins[0][0] > 0 {
		add(0, 0)
	}
	for _, w := range wins {
		if w[0] <= 0 {
			add(0, 1)
		} else {
			add(w[0], 0)
			add(min(w[0]+fade, w[1]), 1)
		}
		if w[1] < a.total {
			add(w[1], 1)
			add(min(w[1]+fade, a.total), 0)
		}
	}
	add(a.total, ks[len(ks)-1].o)
	name := fmt.Sprintf("k%d", a.n)
	a.n++
	fmt.Fprintf(&a.css, "@keyframes %s{", name)
	for _, k := range ks {
		fmt.Fprintf(&a.css, "%s%%{opacity:%s}", a.pct(k.t), num(k.o))
	}
	a.css.WriteString("}\n")
	return name
}

// tokAnim is a value's way through the story.
type tokAnim struct {
	label string
	ks    []tokKey
}

type tokKey struct {
	t float64
	p Pt
	o float64
}

func (tk *tokAnim) add(t float64, p Pt, o float64) {
	if k := len(tk.ks); k > 0 && t <= tk.ks[k-1].t {
		tk.ks[k-1] = tokKey{tk.ks[k-1].t, p, o}
		return
	}
	tk.ks = append(tk.ks, tokKey{t, p, o})
}

func (a *anim) token(t *Tok, frames []frame, j int, t0, t1 []float64) {
	tk := &tokAnim{label: t.label}
	var first *Pt
	for i := 1; i < len(frames); i++ {
		if j < len(frames[i].toks) {
			first = &frames[i].toks[j].path[0]
			break
		}
	}
	if first == nil {
		return
	}
	tk.add(0, *first, 0)
	last := *first
	for i := 1; i < len(frames); i++ {
		if j >= len(frames[i].toks) {
			continue
		}
		f := frames[i].toks[j]
		if !f.alive0 && !f.born {
			tk.add(t0[i], last, 0)
			continue
		}
		p := f.path
		ms := t0[i]
		if f.alive0 {
			tk.add(ms, p[0], 1)
		} else {
			tk.add(ms, p[0], 0)
			ms += fade
			tk.add(ms, p[0], 1)
		}
		for k := 1; k < len(p); k++ {
			tk.add(ms+(t1[i]-ms)*float64(k)/float64(len(p)-1), p[k], 1)
		}
		last = p[len(p)-1]
		tk.add(t1[i], last, 1)
		if !f.alive1 {
			tk.add(t1[i]+0.4, last, 0)
		}
	}
	k := tk.ks[len(tk.ks)-1]
	tk.add(a.total, k.p, k.o)
	a.tokens = append(a.tokens, tk)
}

func (tk *tokAnim) markup(a *anim) string {
	name := fmt.Sprintf("k%d", a.n)
	a.n++
	fmt.Fprintf(&a.css, "@keyframes %s{", name)
	for _, k := range tk.ks {
		fmt.Fprintf(&a.css, "%s%%{transform:translate(%spx,%spx);opacity:%s}", a.pct(k.t), num(k.p.X), num(k.p.Y), num(k.o))
	}
	a.css.WriteString("}\n")
	w := 14 + monoChar*float64(len([]rune(tk.label)))
	return fmt.Sprintf(`<g class="t" style="animation-name:%s"><rect class="tk" x="%s" y="%s" width="%s" height="%s" rx="%s"/><text class="tl m" y="4">%s</text></g>`,
		name, num(-w/2), num(-tokH/2), num(w), num(tokH), num(tokH/2), esc(tk.label))
}

func num(f float64) string {
	return strconv.FormatFloat(float64(int64(f*100+copysign(f)*0.5))/100, 'f', -1, 64)
}

func copysign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

var esc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace

// wrap breaks s into lines of at most width characters.
func wrap(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len([]rune(line))+1+len([]rune(w)) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
