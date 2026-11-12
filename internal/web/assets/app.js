"use strict";

// Logic: pure functions on the page data, testable with node (see logic_test.js).
// Cells are stored row-major in a Uint8Array: 0 free, 1 cross, 2 star.
const Logic = (() => {
  const FREE = 0, CROSS = 1, STAR = 2;

  const idx = (d, r, c) => r * d.n + c;

  // All cells of row, column and region of (r, c) plus its 8 neighbors, without (r, c).
  function implied(d, r, c) {
    const out = new Set();
    for (let i = 0; i < d.n; i++) {
      out.add(idx(d, r, i));
      out.add(idx(d, i, c));
    }
    const reg = d.regions[r][c];
    for (let rr = 0; rr < d.n; rr++)
      for (let cc = 0; cc < d.n; cc++)
        if (d.regions[rr][cc] === reg) out.add(idx(d, rr, cc));
    for (let dr = -1; dr <= 1; dr++)
      for (let dc = -1; dc <= 1; dc++) {
        const rr = r + dr, cc = c + dc;
        if (rr >= 0 && rr < d.n && cc >= 0 && cc < d.n) out.add(idx(d, rr, cc));
      }
    out.delete(idx(d, r, c));
    return [...out];
  }

  // The user's marks plus, if auto, the crosses implied by their stars.
  // Returns {cells, auto}: auto[i] is true for crosses added automatically.
  function effective(d, marks, auto) {
    const cells = Uint8Array.from(marks);
    const isAuto = new Array(cells.length).fill(false);
    if (auto) {
      for (let i = 0; i < cells.length; i++) {
        if (marks[i] !== STAR) continue;
        for (const j of implied(d, Math.floor(i / d.n), i % d.n)) {
          if (cells[j] === FREE) {
            cells[j] = CROSS;
            isAuto[j] = true;
          }
        }
      }
    }
    return { cells, auto: isAuto };
  }

  // Units in the solver's order: rows, columns, regions.
  function units(d) {
    const us = [];
    for (let r = 0; r < d.n; r++) us.push({ kind: "row", index: r, cells: [...Array(d.n).keys()].map((c) => idx(d, r, c)) });
    for (let c = 0; c < d.n; c++) us.push({ kind: "col", index: c, cells: [...Array(d.n).keys()].map((r) => idx(d, r, c)) });
    for (let g = 0; g < d.n; g++) us.push({ kind: "region", index: g, cells: [] });
    for (let r = 0; r < d.n; r++)
      for (let c = 0; c < d.n; c++) us[2 * d.n + d.regions[r][c]].cells.push(idx(d, r, c));
    return us;
  }

  function unitName(d, u) {
    if (u.kind === "row") return "Zeile " + u.index;
    if (u.kind === "col") return "Spalte " + u.index;
    return "Farbe " + d.letters[u.index];
  }

  // Rule violations visible on the board, independent of the solution.
  function problems(d, cells) {
    const out = [];
    for (const u of units(d)) {
      const stars = u.cells.filter((i) => cells[i] === STAR);
      const free = u.cells.filter((i) => cells[i] === FREE);
      if (stars.length > 1) out.push({ msg: `${unitName(d, u)} hat ${stars.length} Sterne`, cells: stars });
      else if (stars.length === 0 && free.length === 0) out.push({ msg: `${unitName(d, u)} hat keinen Platz mehr für einen Stern`, cells: u.cells });
    }
    for (let i = 0; i < cells.length; i++) {
      if (cells[i] !== STAR) continue;
      const r = Math.floor(i / d.n), c = i % d.n;
      // Only look right and down, so each touching pair is reported once.
      for (const [dr, dc] of [[0, 1], [1, -1], [1, 0], [1, 1]]) {
        const rr = r + dr, cc = c + dc;
        if (rr < d.n && cc >= 0 && cc < d.n && cells[idx(d, rr, cc)] === STAR)
          out.push({ msg: `Sterne auf (${r},${c}) und (${rr},${cc}) berühren sich`, cells: [i, idx(d, rr, cc)] });
      }
    }
    return out;
  }

  // Cells whose mark disagrees with the known solution.
  function mistakes(d, cells) {
    if (!d.solution) return [];
    const out = [];
    for (let i = 0; i < cells.length; i++) {
      const isStar = d.solution[Math.floor(i / d.n)] === i % d.n;
      if ((cells[i] === STAR && !isStar) || (cells[i] === CROSS && isStar)) out.push(i);
    }
    return out;
  }

  // Star column per row (-1 for a row without a star), as the party server
  // takes a solution.
  function starColumns(d, cells) {
    const out = [];
    for (let r = 0; r < d.n; r++) {
      let col = -1;
      for (let c = 0; c < d.n; c++) if (cells[idx(d, r, c)] === STAR) col = c;
      out.push(col);
    }
    return out;
  }

  function solved(d, cells) {
    let stars = 0;
    for (const v of cells) if (v === STAR) stars++;
    return stars === d.n && problems(d, cells).length === 0;
  }

  // Applies a step's marks to cells (in place). A conflicting mark (only in the
  // last step of a refuted chain) is skipped.
  function apply(d, cells, step) {
    for (const [r, c, s] of step.marks) {
      const i = idx(d, r, c);
      if (cells[i] === FREE) cells[i] = s ? STAR : CROSS;
    }
    return cells;
  }

  // prefix[k] = board after the first k steps.
  function prefixStates(d, steps) {
    const out = [new Uint8Array(d.n * d.n)];
    for (const st of steps) out.push(apply(d, Uint8Array.from(out[out.length - 1]), st));
    return out;
  }

  // The first trace step that adds something to cells, or -1. All steps before
  // it are already on the board, so its premises hold there too (the rules are
  // monotone: more marks never invalidate a deduction).
  function nextHint(d, cells) {
    for (let k = 0; k < d.steps.length; k++) {
      const fresh = d.steps[k].marks.filter(([r, c]) => cells[idx(d, r, c)] === FREE);
      if (fresh.length > 0) return { step: k, marks: fresh };
    }
    return null;
  }

  // Units and cells named in an explanation, for highlighting:
  // "Zeilen 3 und 4", "Spalten 2 bis 5", "Farben A, B und D", "(2,3)".
  function refs(d, text) {
    const cells = new Set(), unitCells = new Set();
    const addUnit = (kind, i) => {
      if (kind === "row" && i < d.n) for (let c = 0; c < d.n; c++) unitCells.add(idx(d, i, c));
      if (kind === "col" && i < d.n) for (let r = 0; r < d.n; r++) unitCells.add(idx(d, r, i));
      if (kind === "region")
        for (let r = 0; r < d.n; r++) for (let c = 0; c < d.n; c++) if (d.regions[r][c] === i) unitCells.add(idx(d, r, c));
    };
    for (const m of text.matchAll(/\((\d+),(\d+)\)/g)) {
      const r = +m[1], c = +m[2];
      if (r < d.n && c < d.n) cells.add(idx(d, r, c));
    }
    for (const m of text.matchAll(/\b(Zeilen?|Spalten?) (\d+(?:(?:, | und | bis )\d+)*)\b/g)) {
      const kind = m[1].startsWith("Zeile") ? "row" : "col";
      const nums = m[2].split(/, | und /).map(Number);
      if (m[2].includes(" bis ")) {
        const [a, b] = m[2].split(" bis ").map(Number);
        for (let i = a; i <= b; i++) addUnit(kind, i);
      } else nums.forEach((i) => addUnit(kind, i));
    }
    for (const m of text.matchAll(/\b(Farben?) ([A-Za-z](?:(?:, | und )[A-Za-z](?![a-zäöüß]))*)(?![a-zäöüß])/g)) {
      for (const l of m[2].split(/, | und /)) {
        const i = d.letters.indexOf(l);
        if (i >= 0) addUnit("region", i);
      }
    }
    return { cells, units: unitCells };
  }

  // Splits an explanation into text and region references, so the page can
  // show a color swatch instead of the letter: "Farben B und C liegen …" →
  // ["Farben ", {region: 1}, " und ", {region: 2}, " liegen …"].
  function colorSegments(d, text) {
    const out = [];
    let last = 0;
    for (const m of text.matchAll(/\b(Farben?) ([A-Za-z](?:(?:, | und )[A-Za-z](?![a-zäöüß]))*)(?![a-zäöüß])/g)) {
      out.push(text.slice(last, m.index) + m[1] + " ");
      for (const part of m[2].split(/(, | und )/)) {
        const i = d.letters.indexOf(part);
        out.push(part.length === 1 && i >= 0 ? { region: i } : part);
      }
      last = m.index + m[0].length;
    }
    out.push(text.slice(last));
    // Merge neighboring strings, drop empty ones.
    return out.reduce((acc, s) => {
      if (typeof s === "string" && typeof acc[acc.length - 1] === "string") acc[acc.length - 1] += s;
      else acc.push(s);
      return acc;
    }, []).filter((s) => s !== "");
  }

  // German name and color class of a rule, from the step's rule and level.
  function ruleLabel(step) {
    const names = {
      UnitFinish: "Letztes Feld",
      Confinement: "Eingesperrt",
      Elimination: "Ausschluss",
      ForcedPartner: "Erzwungener Partner",
      DeadCells: "Tote Felder",
      Pigeonhole: "Schubfach",
    };
    if (step.rule === "Hypothesis") return { name: step.level >= 101 ? "Raten" : "Annahme", cls: "l5" };
    const cls = step.level <= 2 ? "l1" : step.level <= 4 ? "l2" : step.level < 100 ? "l4" : "l5";
    return { name: names[step.rule] || step.rule, cls };
  }

  // Cells on the straight line from cell a to cell b (both included), so a
  // fast drag that skips cells between two pointer events still covers them.
  function line(d, a, b) {
    const r0 = Math.floor(a / d.n), c0 = a % d.n, r1 = Math.floor(b / d.n), c1 = b % d.n;
    const steps = Math.max(Math.abs(r1 - r0), Math.abs(c1 - c0));
    const out = [];
    for (let t = 0; t <= steps; t++) {
      const f = steps === 0 ? 0 : t / steps;
      const i = idx(d, Math.round(r0 + (r1 - r0) * f), Math.round(c0 + (c1 - c0) * f));
      if (out[out.length - 1] !== i) out.push(i);
    }
    return out;
  }

  // Fat-finger guard: snaps the drag offset (dx, dy in pixels, from where
  // the press on cell start began) to the start cell's row or column,
  // whichever the drag mostly follows, and returns the cell reached there
  // (clipped to the board). Wobbling sideways never leaves the line.
  function snapTarget(d, start, dx, dy, cell) {
    const r = Math.floor(start / d.n), c = start % d.n;
    const clip = (v) => Math.max(0, Math.min(d.n - 1, v));
    if (Math.abs(dx) >= Math.abs(dy)) return idx(d, r, clip(c + Math.round(dx / cell)));
    return idx(d, clip(r + Math.round(dy / cell)), c);
  }

  // Drag painting: crosses on free cells, or (erase) free on crossed cells.
  // Stars are never touched. Changes marks in place, returns whether anything changed.
  function paint(marks, cells, erase) {
    let changed = false;
    for (const i of cells) {
      if (!erase && marks[i] === FREE) { marks[i] = CROSS; changed = true; }
      if (erase && marks[i] === CROSS) { marks[i] = FREE; changed = true; }
    }
    return changed;
  }

  // Coop rounds share one board on the server. A change is an operation
  // [num, i, from, to]: cell i, seen as from, becomes to. It applies if the
  // cell still is from, is fine if it already is to, and is rejected
  // otherwise, so a change never overwrites one made meanwhile by somebody
  // else (the same rule as the server, coop.go).
  // applyOps returns a copy of cells with ops applied and the numbers of the
  // rejected ones.
  function applyOps(cells, ops) {
    const out = Uint8Array.from(cells), rejected = [];
    for (const [num, i, from, to] of ops) {
      if (out[i] === from) out[i] = to;
      else if (out[i] !== to) rejected.push(num);
    }
    return { cells: out, rejected };
  }

  // The changes from before to after as [i, from, to].
  function diffOps(before, after) {
    const out = [];
    for (let i = 0; i < after.length; i++) if (before[i] !== after[i]) out.push([i, before[i], after[i]]);
    return out;
  }

  // Region colors: 12 hand-tuned colors [hue, saturation, lightness] for the
  // light theme (dark: same hue, less saturation, low lightness). Fewer regions
  // use an evenly spread subset; more regions fall back to computed hues. The
  // stride shuffles the order, so regions with consecutive letters (often
  // neighbors on the board) do not get neighboring colors.
  const PALETTE = [
    [0, 72, 82], [28, 85, 76], [50, 85, 70], [85, 50, 72], [140, 40, 76], [176, 45, 68],
    [202, 70, 80], [228, 65, 83], [266, 55, 84], [305, 40, 80], [35, 28, 72], [0, 0, 80],
  ];
  const gcd = (a, b) => (b ? gcd(b, a % b) : a);
  const regionHSL = (n, reg) => {
    let stride = Math.max(1, Math.round(n * 0.382));
    while (gcd(stride, n) !== 1) stride++;
    const rank = (reg * stride) % n;
    if (n <= PALETTE.length) return PALETTE[Math.floor((rank * PALETTE.length) / n)];
    return [Math.round(rank * (360 / n) + 25) % 360, 60, rank % 2 ? 74 : 82];
  };

  // The library puzzle after the one with ID id, in the overview's order (by
  // size, then number; wrapping around), that is not in solved (an object
  // keyed by ID), or null if all are solved.
  function nextUnsolved(entries, solved, id) {
    const sorted = [...entries].sort((a, b) => a.n - b.n || a.nr - b.nr);
    const i = sorted.findIndex((e) => e.id === id);
    const after = sorted.slice(i + 1).concat(sorted.slice(0, i + 1));
    return after.find((e) => e.id !== id && !(solved && Object.hasOwn(solved, e.id))) || null;
  }

  // The server's board string ("0120…") as cells.
  const decodeBoard = (str) => Uint8Array.from(str, (ch) => ch.charCodeAt(0) - 48);

  // ---------- play options ----------
  // On/off options of this device, set on /einstellungen (or, in a page saved
  // as a file, on the page itself). Unset, they take their default; the
  // fat-finger guard is on by default for touch screens.
  const PREFS = {
    autocross: { key: "starbattle.autocross", def: () => false },
    straight: { key: "starbattle.straight", def: () => typeof matchMedia === "function" && matchMedia("(pointer: coarse)").matches },
    showProblems: { key: "starbattle.showProblems", def: () => true },
  };
  function pref(name) {
    try {
      const v = localStorage.getItem(PREFS[name].key);
      if (v === "1" || v === "0") return v === "1";
    } catch { /* private mode etc. */ }
    return PREFS[name].def();
  }
  const setPref = (name, on) => {
    try { localStorage.setItem(PREFS[name].key, on ? "1" : "0"); } catch { /* private mode etc. */ }
  };
  // The option a storage key belongs to, if any (for changes from other tabs).
  const prefOfKey = (key) => Object.keys(PREFS).find((k) => PREFS[k].key === key) || null;

  // ---------- star symbol ----------
  // The symbol chosen on /einstellungen, kept in the browser's storage as a
  // copy of its catalog entry (symbols.go), so pages draw it without asking
  // the server. markSpec checks such an entry and falls back to the star.
  const MARK_KEY = "starbattle.symbol";
  const DEFAULT_MARK = Object.freeze({
    id: "star", kind: "svg", tint: true, scale: 1, frames: 1, fps: 0,
    path: "M50 6L61.5 37.2 94.7 38.5 68.5 59 77.6 91 50 72.5 22.4 91 31.5 59 5.3 38.5 38.5 37.2Z",
  });
  const PATH_RE = /^[MmLlHhVvCcSsQqTtAaZz0-9 ,.eE+-]+$/;
  const URL_RE = /^\/symbole\/[a-z0-9-]+\.(svg|png|webp)(\?v=[0-9a-f]+)?$/;

  // A drawable spec {id, kind, tint, scale, frames, fps, clips, path|glyph|url}
  // for a stored entry. Sprites need the server: with opts.sprites false (a page
  // opened as a file) they fall back to the star, as does anything broken.
  // The choice "random" picks one by chance (see randomMark).
  function markSpec(e, opts) {
    if (!e || typeof e !== "object") return DEFAULT_MARK;
    if (e.kind === "random") return randomMark(e, opts);
    const id = typeof e.id === "string" ? e.id : "";
    const scale = Number.isFinite(e.scale) && e.scale > 0 && e.scale <= 2 ? e.scale : 1;
    const base = { id, scale, frames: 1, fps: 0 };
    switch (e.kind) {
      case "svg":
        if (typeof e.path === "string" && PATH_RE.test(e.path)) return { ...base, kind: "svg", tint: true, path: e.path };
        break;
      case "glyph":
      case "emoji":
        if (typeof e.glyph === "string" && e.glyph.length > 0 && e.glyph.length <= 24) return { ...base, kind: e.kind, tint: e.kind === "glyph", glyph: e.glyph };
        break;
      case "sprite": {
        if (!(opts && opts.sprites) || typeof e.url !== "string" || !URL_RE.test(e.url)) break;
        const frames = Number.isInteger(e.frames) && e.frames >= 1 && e.frames <= 64 ? e.frames : 1;
        const fps = frames > 1 && Number.isInteger(e.fps) && e.fps >= 1 && e.fps <= 30 ? e.fps : 0;
        if (!fps) return { ...base, kind: "sprite", tint: false, url: e.url };
        return { ...base, kind: "sprite", tint: false, url: e.url, frames, fps, clips: markClips(e.clips, frames) };
      }
    }
    return DEFAULT_MARK;
  }

  // The choice "random" on /einstellungen: {id: "random", kind: "random",
  // pool: catalog entries}. A page picks one of the pool that it can draw,
  // with opts.rnd (default Math.random), if possible not the one with the ID
  // opts.not. It keeps it until "Neu beginnen" or a reload.
  const RANDOM_MARK = "random";
  const MAX_POOL = 64;
  function randomMark(e, opts) {
    if (!Array.isArray(e.pool)) return DEFAULT_MARK;
    const o = opts || {};
    let usable = e.pool.slice(0, MAX_POOL)
      .filter((x) => x && typeof x === "object" && x.kind !== "random")
      .map((x) => markSpec(x, o))
      .filter((x) => x !== DEFAULT_MARK);
    if (usable.some((x) => x.id !== o.not)) usable = usable.filter((x) => x.id !== o.not);
    if (usable.length === 0) return DEFAULT_MARK;
    return usable[Math.floor((o.rnd || Math.random)() * usable.length) % usable.length];
  }
  const isRandomMark = (e) => !!e && e.kind === "random";

  // The clips {start, frames} of an animated sprite (see Clip in symbols.go):
  // the given ones if all fit, else one of all frames.
  function markClips(clips, frames) {
    const ok = Array.isArray(clips) && clips.length >= 1 && clips.length <= 16 && clips.every((c) =>
      c && Number.isInteger(c.start) && Number.isInteger(c.frames) && c.start >= 0 && c.frames >= 1 && c.start + c.frames <= frames);
    return ok ? clips.map((c) => ({ start: c.start, frames: c.frames })) : [{ start: 0, frames }];
  }

  const loadMark = () => {
    try { return JSON.parse(localStorage.getItem(MARK_KEY) || "null"); } catch { return null; }
  };
  const saveMark = (e) => {
    try { localStorage.setItem(MARK_KEY, JSON.stringify(e)); } catch { /* private mode etc. */ }
  };

  const GLYPH_FONT = '"Segoe UI Symbol", "DejaVu Sans", system-ui, sans-serif';
  const EMOJI_FONT = '"Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", system-ui, sans-serif';

  // Where a glyph goes so that its ink, as this device's font draws it, sits
  // in the middle of the 100×100 box at 80 units: font metrics put glyphs off
  // center by different amounts on every system. Null if it cannot be measured.
  function glyphBox(glyph, font, doc) {
    try {
      const ctx = doc.createElement("canvas").getContext("2d");
      ctx.font = "100px " + font;
      const m = ctx.measureText(glyph);
      const l = m.actualBoundingBoxLeft, r = m.actualBoundingBoxRight, a = m.actualBoundingBoxAscent, d = m.actualBoundingBoxDescent;
      const w = l + r, h = a + d;
      if (![l, r, a, d].every(Number.isFinite) || w <= 1 || h <= 1) return null;
      const k = 80 / Math.max(w, h);
      const f = (x) => Math.round(x * 100) / 100;
      return { x: f(50 - ((r - l) / 2) * k), y: f(50 - ((d - a) / 2) * k), "font-size": f(100 * k) };
    } catch {
      return null;
    }
  }

  // The symbol layer of a cell: the mark of spec (class g-star) plus cross and
  // auto-cross dot, in a 100×100 box. CSS shows the one matching the cell's
  // class. Built with DOM calls, never markup, as the entry comes from storage.
  function markSVG(spec, doc) {
    const NS = "http://www.w3.org/2000/svg";
    const mk = (tag, attrs) => {
      const e = doc.createElementNS(NS, tag);
      for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
      return e;
    };
    const svg = mk("svg", { viewBox: "0 0 100 100", "aria-hidden": "true" });
    const g = mk("g", { class: "g-star g-" + spec.kind });
    if (spec.scale !== 1) g.setAttribute("transform", `translate(50 50) scale(${spec.scale}) translate(-50 -50)`);
    if (spec.kind === "svg") g.append(mk("path", { d: spec.path }));
    else if (spec.kind === "glyph" || spec.kind === "emoji") {
      const font = spec.kind === "emoji" ? EMOJI_FONT : GLYPH_FONT;
      const t = mk("text", glyphBox(spec.glyph, font, doc) || { x: 50, y: 54, "text-anchor": "middle", "dominant-baseline": "central", "font-size": 84 });
      t.style.fontFamily = font;
      t.textContent = spec.glyph;
      g.append(t);
    } else if (spec.frames > 1) {
      // Frames side by side, shown through a 100×100 window and moved by CSS.
      const win = mk("svg", { x: 0, y: 0, width: 100, height: 100, viewBox: "0 0 100 100" });
      win.append(mk("image", { class: "g-frames", href: spec.url, width: 100 * spec.frames, height: 100, preserveAspectRatio: "none" }));
      g.append(win);
    } else g.append(mk("image", { href: spec.url, width: 100, height: 100 }));
    svg.append(g, mk("path", { class: "g-cross", d: "M32 32L68 68M68 32L32 68" }), mk("circle", { class: "g-auto", cx: 50, cy: 50, r: 8 }));
    return svg;
  }

  // Classes for spec on the element around the cells.
  function markClasses(el, spec) {
    el.classList.toggle("untinted", !spec.tint);
    el.classList.toggle("mark-emoji", spec.kind === "emoji");
  }

  // An animated mark plays when set: one of its clips, whole runs for about
  // MARK_PLAY_S seconds, and then it stays on its first frame.
  const MARK_PLAY_S = 3;
  const markRuns = (clip, fps) => (fps > 0 ? Math.max(1, Math.round((MARK_PLAY_S * fps) / clip.frames)) : 0);

  // A random index below n, not prev if there is another choice.
  function pickClip(n, prev, rnd) {
    if (n <= 1) return 0;
    if (!(prev >= 0 && prev < n)) return Math.floor(rnd() * n) % n;
    const k = Math.floor(rnd() * (n - 1)) % (n - 1);
    return k >= prev ? k + 1 : k;
  }

  // Plays the mark of spec in the cell el (class fresh, see style.css), if it
  // is animated: a clip at random, if possible another than the last one.
  let lastClip = -1;
  function playMark(el, spec, rnd = Math.random) {
    el.classList.remove("fresh");
    if (!(spec.frames > 1)) return;
    const k = pickClip(spec.clips.length, lastClip, rnd);
    lastClip = k;
    const c = spec.clips[k];
    el.style.setProperty("--clip-from", -100 * c.start + "px");
    el.style.setProperty("--clip-to", -100 * (c.start + c.frames) + "px");
    el.style.setProperty("--clip-frames", c.frames);
    el.style.setProperty("--clip-dur", c.frames / spec.fps + "s");
    el.style.setProperty("--clip-runs", markRuns(c, spec.fps));
    void el.offsetWidth; // restart a running animation
    el.classList.add("fresh");
  }

  return { FREE, CROSS, STAR, idx, implied, effective, units, problems, mistakes, starColumns, solved, apply, prefixStates, nextHint, refs, ruleLabel, line, snapTarget, paint, colorSegments, applyOps, diffOps, decodeBoard, regionHSL, nextUnsolved, pref, setPref, prefOfKey, MARK_KEY, DEFAULT_MARK, RANDOM_MARK, markSpec, isRandomMark, loadMark, saveMark, markSVG, markClasses, markRuns, pickClip, playMark };
})();

if (typeof module !== "undefined") module.exports = Logic;
if (typeof document !== "undefined" && document.getElementById("board")) boot();

function boot() {
  const L = Logic;
  const d = JSON.parse(document.getElementById("data").textContent);
  const n = d.n;
  const $ = (id) => document.getElementById(id);
  const el = (tag, cls, text) => {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };

  const regionColor = (e, reg) => {
    const [h, s, l] = L.regionHSL(n, reg);
    e.style.setProperty("--h", h);
    e.style.setProperty("--s", s + "%");
    e.style.setProperty("--l", l + "%");
    e.style.setProperty("--ds", Math.round(s * 0.62) + "%");
    e.style.setProperty("--dl", Math.round(22 + (l - 60) * 0.75) + "%");
  };
  // Explanation text with a color swatch for every "Farbe X".
  const rich = (text) =>
    L.colorSegments(d, text).map((s) => {
      if (typeof s === "string") return document.createTextNode(s);
      const sw = el("span", "sw");
      regionColor(sw, s.region);
      sw.title = "Farbe " + d.letters[s.region];
      sw.setAttribute("role", "img");
      sw.setAttribute("aria-label", sw.title);
      sw.dataset.region = s.region;
      return sw;
    });
  const elr = (tag, cls, text) => {
    const e = el(tag, cls);
    e.append(...rich(text));
    return e;
  };

  // ---------- board ----------
  // Marks as vector shapes sized to the cell (font glyphs were small and thin),
  // the star replaced by the symbol chosen on /einstellungen; CSS shows the
  // one matching the cell's class. Image symbols need the server.
  const markOpts = { sprites: /^https?:$/.test(location.protocol) };
  let mark = L.markSpec(L.loadMark(), markOpts);
  const symTpl = L.markSVG(mark, document);
  const boardEl = $("board");
  boardEl.style.setProperty("--n", n);
  L.markClasses(boardEl, mark);
  boardEl.append(el("div", "axis"));
  for (let c = 0; c < n; c++) boardEl.append(el("div", "axis top", c));
  const cellEls = [];
  for (let r = 0; r < n; r++) {
    boardEl.append(el("div", "axis", r));
    for (let c = 0; c < n; c++) {
      const reg = d.regions[r][c];
      const e = el("div", "cell");
      regionColor(e, reg);
      const sym = el("span", "sym");
      sym.append(symTpl.cloneNode(true));
      e.append(sym);
      e.dataset.i = r * n + c;
      e.setAttribute("aria-label", `Feld (${r},${c}), Farbe ${d.letters[reg]}`);
      boardEl.append(e);
      cellEls.push(e);
    }
  }

  // Grid lines and region walls as one SVG layer over the cells, so every
  // cell has the same size (borders on the cells made those at walls smaller,
  // and their stars with them) and walls meet in clean corners: each wall is
  // one path of maximal straight runs with square caps.
  {
    const wall = (r0, c0, r1, c1) => r0 < 0 || c0 < 0 || r1 >= n || c1 >= n || d.regions[r0][c0] !== d.regions[r1][c1];
    let thin = "", thick = "";
    for (let k = 0; k <= n; k++) {
      if (k > 0 && k < n) thin += `M0 ${k}H${n}M${k} 0V${n}`;
      // Horizontal line k (above row k), then vertical line k (left of column k).
      for (const horiz of [true, false]) {
        let from = -1;
        for (let j = 0; j <= n; j++) {
          const on = j < n && (horiz ? wall(k - 1, j, k, j) : wall(j, k - 1, j, k));
          if (on && from < 0) from = j;
          if (!on && from >= 0) {
            thick += horiz ? `M${from} ${k}H${j}` : `M${k} ${from}V${j}`;
            from = -1;
          }
        }
      }
    }
    const walls = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    walls.setAttribute("class", "walls");
    walls.setAttribute("viewBox", `0 0 ${n} ${n}`);
    walls.setAttribute("preserveAspectRatio", "none");
    walls.setAttribute("aria-hidden", "true");
    walls.innerHTML = `<path class="thin" d="${thin}"/><path class="thick" d="${thick}"/>`;
    walls.style.gridArea = `2 / 2 / ${n + 2} / ${n + 2}`;
    boardEl.append(walls);
  }

  // view: {cells, auto?, marks: {new, hint, bad, refcell, ref} as Sets of indices}
  // An animated star symbol plays only for stars just set by a click, a hint
  // or a partner: not on loading, nor when many appear at once (switching to
  // the solution, redo of a reset).
  let drawnOnce = false;
  function drawBoard(view) {
    const m = view.marks || {};
    let fresh = 0;
    for (let i = 0; i < cellEls.length; i++) if (view.cells[i] === L.STAR && !cellEls[i].classList.contains("star")) fresh++;
    const play = drawnOnce && fresh <= 2;
    drawnOnce = true;
    for (let i = 0; i < cellEls.length; i++) {
      const e = cellEls[i], v = view.cells[i];
      const auto = view.auto && view.auto[i];
      if (v !== L.STAR) e.classList.remove("fresh");
      else if (!e.classList.contains("star")) {
        if (play) L.playMark(e, mark);
        else e.classList.remove("fresh");
      }
      e.classList.toggle("star", v === L.STAR);
      e.classList.toggle("cross", v === L.CROSS && !auto);
      e.classList.toggle("auto", !!auto);
      for (const k of ["new", "hint", "bad", "refcell"]) e.classList.toggle(k, !!(m[k] && m[k].has(i)));
      // Cells outside the units named in the explanation are dimmed.
      e.classList.toggle("dim", !!(m.ref && m.ref.size && !m.ref.has(i)));
    }
  }

  function banner(text, cls) {
    const b = $("banner");
    b.hidden = !text;
    b.replaceChildren(...rich(text || ""));
    b.className = "banner " + (cls || "");
  }

  function chip(step) {
    const lab = L.ruleLabel(step);
    const c = el("span", "chip " + lab.cls, lab.name);
    c.title = `${step.source} (Stufe ${step.level})`;
    return c;
  }

  // Pointing at a color swatch in a text spotlights that region on the board
  // (tap on touch screens toggles it).
  function spotlight(reg) {
    boardEl.classList.toggle("spotting", reg != null);
    cellEls.forEach((e, i) => e.classList.toggle("spot", reg != null && d.regions[Math.floor(i / n)][i % n] === reg));
  }
  let spotted = null;
  document.addEventListener("pointerover", (ev) => {
    const sw = ev.target.closest && ev.target.closest(".sw[data-region]");
    if (sw && ev.pointerType === "mouse") spotlight(+sw.dataset.region);
  });
  document.addEventListener("pointerout", (ev) => {
    const sw = ev.target.closest && ev.target.closest(".sw[data-region]");
    if (sw && ev.pointerType === "mouse") spotlight(spotted);
  });
  document.addEventListener("click", (ev) => {
    const sw = ev.target.closest && ev.target.closest(".sw[data-region]");
    if (!sw) {
      if (spotted != null) spotlight((spotted = null));
      return;
    }
    if (ev.pointerType === "mouse") return; // the mouse spotlights on hover
    const reg = +sw.dataset.region; // the click still reaches the step row (navigates)
    spotted = spotted === reg ? null : reg;
    spotlight(spotted);
  }, true);

  // ---------- summary ----------
  function drawSummary() {
    if (d.library && d.library.pending) { // the solver's result comes later
      $("summary").textContent = `${n}×${n} · ohne Hinweise`;
      return;
    }
    const k = d.steps.length;
    const statusText =
      { solved: `gelöst in ${k} Schritten`, stuck: `steckt nach ${k} Schritten fest`, contradiction: `Widerspruch nach ${k} Schritten` }[d.status] || d.status;
    let summary = `${n}×${n} · Solver: ${statusText}`;
    if (d.stats.lookahead) summary += ` · ${d.stats.lookahead} Annahmen`;
    if (d.stats.guesses) summary += ` · ${d.stats.guesses}× geraten`;
    if (!d.solution) summary += " · keine eindeutige Lösung";
    $("summary").textContent = summary;
  }
  drawSummary();

  // ---------- storage (per-viewer convenience only) ----------
  // A party round and a library puzzle keep their own progress, apart from
  // solo play of the puzzle. Library puzzles go by their stable ID.
  const key = d.library
    ? "starbattle:library:" + d.library.id
    : (d.party ? `starbattle:party:${d.party.code}:${d.party.round}:` : "starbattle:") + n + ":" + d.regions.map((row) => row.join(",")).join(";");
  const load = () => {
    try { return JSON.parse(localStorage.getItem(key) || "null"); } catch { return null; }
  };
  const save = (v) => {
    try { localStorage.setItem(key, JSON.stringify(v)); } catch { /* private mode etc. */ }
  };

  // ---------- play mode ----------
  const saved = load();
  let marks = new Uint8Array(n * n);
  if (saved && Array.isArray(saved.marks) && saved.marks.length === n * n) marks = Uint8Array.from(saved.marks);
  // Options of this device (see L.pref), set on /einstellungen. A page saved
  // as a file cannot reach that page and shows them itself.
  const prefs = { autocross: L.pref("autocross"), straight: L.pref("straight"), showProblems: L.pref("showProblems") };

  // Solving time and hints shown, saved with the marks. The clock runs only
  // while the page is visible and stops for good once the puzzle is solved.
  const num = (v) => (Number.isFinite(v) && v >= 0 ? v : 0);
  let elapsed = num(saved && saved.time); // ms, not counting the running stretch
  let hintCount = num(saved && saved.hints);
  let done = !!(saved && saved.done);
  let runningSince = null;
  const clockMs = () => elapsed + (runningSince === null ? 0 : Math.max(0, Date.now() - runningSince));
  function clockRun(on) {
    if (party) return; // a party round runs on the server's clock (partySnap)
    if (on && runningSince === null && !done) runningSince = Date.now();
    if (!on) clockStop();
  }
  function clockStop() {
    if (runningSince !== null) { elapsed = clockMs(); runningSince = null; }
  }
  // A round played together (d.party), see "party mode" below.
  const party = d.party ? { gaveUp: !!(saved && saved.gaveUp), snap: null, startLocal: null, live: false, solving: false, sentStars: -1, sentHints: -1 } : null;
  // A coop round (all on one board, kept by the server), see "coop mode" below.
  const coop = party && d.party.mode === "coop"
    ? { server: new Uint8Array(n * n), version: -1, pending: [], nextNum: 0, ready: false, sending: false, voted: false, lost: new Set() }
    : null;
  if (coop) { // the board comes from the server, nothing from storage
    marks = new Uint8Array(n * n);
    done = party.gaveUp = false;
    hintCount = 0;
  }
  // A library puzzle is solved without help: no hints, no checks, and the
  // solver's way only once it is solved. Solved IDs are kept on this device
  // for the overview (/bibliothek), with the best time.
  const lib = d.library || null;
  const libSolvedKey = "starbattle:library:solved";
  function libSolved() {
    try {
      const v = JSON.parse(localStorage.getItem(libSolvedKey) || "{}");
      return v && typeof v === "object" ? v : {};
    } catch { return {}; }
  }
  function libRecord(ms) {
    const all = libSolved();
    const prev = all[lib.id];
    if (prev && Number.isFinite(prev.time) && prev.time <= ms) return;
    all[lib.id] = { time: Math.round(ms), at: new Date().toISOString().slice(0, 10) };
    try { localStorage.setItem(libSolvedKey, JSON.stringify(all)); } catch { /* private mode etc. */ }
  }
  if (lib) {
    $("hint").hidden = true;
    $("check").hidden = true;
    $("play-msg").append(el("p", "muted", "Bibliothek: ein besonders schweres Puzzle, an dem die einfachen Regeln scheitern. Hier gibt es keine Hinweise und kein Prüfen. Den Lösungsweg des Solvers siehst du erst, wenn du das Puzzle gelöst hast."));
  }
  // No input: solved, or in a party round given up, not started or not ours.
  const locked = () => done || (!!party && (party.gaveUp || !partyLive()));
  function fmtTime(ms) {
    const s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    const ss = String(s % 60).padStart(2, "0");
    return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
  }
  const hintsText = (k) => (k === 1 ? "1 Hinweis" : `${k} Hinweise`);
  function drawClock() {
    $("clock").textContent = coop ? `⏱ ${fmtTime(clockMs())} · gemeinsam` : lib ? `⏱ ${fmtTime(clockMs())}` : `⏱ ${fmtTime(clockMs())} · ${hintsText(hintCount)}`;
  }
  const undoStack = [], redoStack = [];
  let overlay = null; // {hint?, bad?} shown until the next change
  const intro = [...$("play-msg").childNodes].map((x) => x.cloneNode(true));

  function setMarks(next) {
    if (locked()) return; // solved puzzles are frozen
    if (coop) {
      coopCommit(L.diffOps(marks, next));
      return;
    }
    undoStack.push(marks);
    redoStack.length = 0;
    marks = next;
    overlay = null;
    renderPlay();
  }

  function clickCell(i, star) {
    const next = Uint8Array.from(marks);
    if (star) next[i] = marks[i] === L.STAR ? L.FREE : L.STAR;
    else next[i] = (marks[i] + 1) % 3; // free → cross → star → free
    setMarks(next);
  }

  function renderPlay() {
    const eff = L.effective(d, marks, prefs.autocross);
    // Rule violations (two stars touching, a full row …) unless switched off;
    // "Prüfen" still marks wrong cells, as asked for explicitly.
    const probs = prefs.showProblems ? L.problems(d, eff.cells) : [];
    const bad = new Set(probs.flatMap((p) => p.cells));
    const view = { cells: eff.cells, auto: eff.auto, marks: { bad } };
    if (overlay) Object.assign(view.marks, overlay);
    drawBoard(party && party.gaveUp ? solutionView() : view);

    const list = $("problems");
    list.replaceChildren(...probs.map((p) => elr("li", "", p.msg)));
    // Once solved, the puzzle is frozen until "Neu beginnen": the board
    // ignores input and scrolls on touch like the rest of the page.
    const frozen = locked() || L.solved(d, eff.cells);
    document.body.classList.toggle("frozen", frozen);
    if (party) {
      document.body.classList.toggle("party-wait", !party.gaveUp && !done && !partyStarted());
      $("giveup").disabled = frozen;
      $("reset").disabled = frozen;
      if (coop) {
        $("giveup").textContent = coop.voted ? "Doch nicht aufgeben" : "Aufgeben";
        $("giveup").classList.toggle("voted", coop.voted);
      }
    }
    $("undo").disabled = frozen || undoStack.length === 0;
    $("redo").disabled = frozen || redoStack.length === 0;
    $("hint").disabled = frozen;
    $("check").disabled = frozen;
    $("hint").textContent = overlay && overlay.hint ? "Hinweis ausblenden" : "Hinweis";
    // In a coop round the server says when the board is solved.
    if (L.solved(d, eff.cells) && !(party && party.gaveUp) && (!coop || done)) {
      if (!done) {
        clockStop();
        done = true;
        if (party) partySolve();
      }
      if (lib) libRecord(clockMs());
      const with_ = lib ? "ohne Hilfe" : !hintCount ? "ohne Hinweise" : hintCount === 1 ? "mit 1 Hinweis" : `mit ${hintCount} Hinweisen`;
      banner(coop ? `Gemeinsam gelöst in ${fmtTime(clockMs())}! Alle Sterne sitzen richtig.` : `Gelöst in ${fmtTime(clockMs())} ${with_}! Alle Sterne sitzen richtig.`, "good");
      if (!party) {
        const share = el("button", "share", "Ergebnis kopieren");
        share.title = lib ? "Text mit Zeit und Link zu diesem Puzzle in die Zwischenablage" : "Text mit Zeit, Hinweisen und Link in die Zwischenablage";
        share.onclick = () => copyText(shareText(with_), share);
        $("banner").append(" ", share);
      }
      if (lib) {
        const next = L.nextUnsolved(lib.entries, libSolved(), lib.id);
        const a = el("a", "next", next ? `Weiter: ${next.name}` : "Alle gelöst: zur Bibliothek");
        a.href = next ? "/bibliothek?id=" + encodeURIComponent(next.id) : "/bibliothek";
        $("banner").append(" ", a);
      }
    } else if (party) {
      const b = partyBanner();
      banner(b && b[0], b && b[1]);
    } else banner(null);
    drawClock();
    store();
    if (party) {
      partyTabs();
      partyReport();
    }
    if (lib) {
      $("tab-watch").hidden = !done;
      if (done && lib.pending) loadTrace();
    }
  }

  // A library page may come before the solver is done (lib.pending): the
  // solution and trace are fetched once the puzzle is solved by hand. The
  // request waits for the solver; meanwhile the "Lösungsweg" tab shows a
  // spinner.
  function loadTrace() {
    if (lib.loading) return;
    lib.loading = true;
    $("tab-watch").classList.add("loading");
    $("trace-wait").replaceChildren(el("span", "spinner"), " Der Solver rechnet noch am Lösungsweg …");
    fetch("/bibliothek/weg?id=" + encodeURIComponent(lib.id))
      .then((r) => (r.ok ? r.json() : r.text().then((t) => Promise.reject(new Error(t.trim() || r.statusText)))))
      .then((t) => {
        for (const f of ["solution", "status", "error", "stats", "steps"]) d[f] = t[f];
        lib.pending = false;
        setTrace();
        drawSummary();
      })
      .catch((e) => {
        const again = el("button", "", "Nochmal versuchen");
        again.onclick = loadTrace;
        $("trace-wait").replaceChildren(el("p", "", "Der Lösungsweg ließ sich nicht laden: " + e.message), again);
      })
      .finally(() => {
        lib.loading = false;
        $("tab-watch").classList.remove("loading");
        if (mode === "watch") render();
      });
  }

  // The text for sharing a result; with the page's link when it has a stable
  // one (served pages: /?n=8&seed=42 or /play?p=...), not for a local file.
  function shareText(with_) {
    const web = d.served && /^https?:$/.test(location.protocol);
    if (lib) {
      let t = `Schweres Star Battle ${lib.name} (${n}×${n}) aus der Bibliothek in ${fmtTime(clockMs())} gelöst, ${with_}!`;
      if (web) t += " " + location.origin + "/bibliothek?id=" + encodeURIComponent(lib.id);
      return t;
    }
    let t = `Star Battle ${n}×${n} in ${fmtTime(clockMs())} gelöst (${with_})!`;
    if (web) t += " " + location.href.split("#")[0];
    return t;
  }

  // Copies text to the clipboard. navigator.clipboard needs a secure context
  // (HTTPS or localhost), which a home server like http://raspberrypi:6369 is
  // not, so fall back to the old execCommand route.
  async function copyText(text, btn) {
    let ok = false;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        ok = true;
      }
    } catch { /* fall through */ }
    if (!ok) {
      const ta = el("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.append(ta);
      ta.focus();
      ta.select();
      try { ok = document.execCommand("copy"); } catch { ok = false; }
      ta.remove();
    }
    if (ok) {
      btn.textContent = "Kopiert ✓";
      setTimeout(() => { btn.textContent = "Ergebnis kopieren"; }, 2000);
    } else {
      window.prompt("Kopieren nicht möglich, bitte selbst kopieren:", text);
    }
  }

  function store() {
    if (coop) return; // the board lives on the server
    const v = { marks: [...marks], time: Math.round(clockMs()), hints: hintCount, done, paid: [...paid] };
    if (party) v.gaveUp = party.gaveUp;
    save(v);
  }

  function message(...nodes) {
    $("play-msg").replaceChildren(...nodes);
  }

  // The hint button toggles: a second click hides the hint again.
  function toggleHint() {
    if (overlay && overlay.hint) {
      overlay = null;
      message(...intro.map((x) => x.cloneNode(true)));
      renderPlay();
    } else showHint();
  }

  function showHint() {
    const eff = L.effective(d, marks, prefs.autocross);
    const wrong = L.mistakes(d, eff.cells);
    if (wrong.length) {
      message(el("p", "", "Etwas passt nicht zur Lösung. Erst das korrigieren, dann gibt es den nächsten Hinweis."), showMistakesButton());
      return;
    }
    const h = L.nextHint(d, eff.cells);
    if (!h) {
      message(el("p", "", L.solved(d, eff.cells) ? "Schon gelöst." : "Hier kommt der Solver auch nicht weiter."));
      return;
    }
    const step = d.steps[h.step];
    const refs = L.refs(d, step.reason);
    if (!done) hintCount++;
    overlay = { hint: new Set(h.marks.map(([r, c]) => L.idx(d, r, c))), ref: refs.units, refcell: refs.cells };
    const head = el("p", "");
    head.append(chip(step), " ", el("span", "muted", `Schritt ${h.step + 1} des Lösungswegs`));
    const reason = elr("p", "", step.reason);
    const nodes = [head, reason];
    if (step.hypothesis) {
      const det = el("details");
      det.append(el("summary", "", "Gedankengang anzeigen"));
      const ol = el("ol");
      for (const s of step.hypothesis.steps) ol.append(elr("li", "", s.reason));
      det.append(ol, elr("p", "contra", "⇒ Widerspruch: " + step.hypothesis.contradiction));
      nodes.push(det);
    }
    const apply = el("button", "primary", "Anwenden");
    apply.onclick = () => {
      const next = Uint8Array.from(marks);
      for (const [r, c, s] of h.marks) next[L.idx(d, r, c)] = s ? L.STAR : L.CROSS;
      setMarks(next);
      message(el("p", "muted", "Angewendet. Für den nächsten Schritt wieder „Hinweis“."));
    };
    const a = el("div", "actions");
    a.append(apply);
    nodes.push(a);
    message(...nodes);
    renderPlay();
  }

  // Checking is free and only says whether the marks so far agree with the
  // solution. Showing where they don't costs one hint per wrongly set field
  // (auto-crosses next to a wrong star are shown for free); fields already
  // paid for in their current state are not charged again.
  const paid = new Set(saved && Array.isArray(saved.paid) ? saved.paid : []); // "i:mark" of wrong fields already shown
  function checkMistakes() {
    if (!d.solution) {
      message(el("p", "", "Für dieses Puzzle ist keine eindeutige Lösung bekannt."));
      return;
    }
    overlay = null;
    const eff = L.effective(d, marks, prefs.autocross);
    if (!L.mistakes(d, eff.cells).length) message(el("p", "", L.solved(d, eff.cells) ? "Gelöst." : "Bisher alles richtig."));
    else message(el("p", "", "Da stimmt etwas nicht: mindestens ein Feld passt nicht zur Lösung."), showMistakesButton());
    renderPlay();
  }

  function showMistakesButton() {
    const b = el("button", "", "Fehler zeigen");
    b.title = "Kostet einen Hinweis pro falsch gesetztem Feld";
    b.onclick = showMistakes;
    const a = el("div", "actions");
    a.append(b, " ", el("span", "muted", "kostet 1 Hinweis pro falschem Feld"));
    return a;
  }

  function showMistakes() {
    const eff = L.effective(d, marks, prefs.autocross);
    const wrong = L.mistakes(d, eff.cells);
    let cost = 0;
    for (const i of L.mistakes(d, marks)) {
      const k = i + ":" + marks[i];
      if (!paid.has(k)) { paid.add(k); cost++; }
    }
    if (!done) hintCount += cost;
    overlay = { bad: new Set(wrong) };
    const what = `${wrong.length === 1 ? "Ein Feld ist" : wrong.length + " Felder sind"} falsch (rot markiert).`;
    message(el("p", "", cost ? `${what} ${cost === 1 ? "1 Hinweis" : cost + " Hinweise"} angerechnet.` : what));
    renderPlay();
  }

  // Pointer input: a press without moving to another cell is a click (cycle
  // free → cross → star). Moving on paints crosses over every cell passed, or
  // removes them if the press started on a cross. One drag is one undo step.
  // With the fat-finger guard on, a drag draws one straight line (row or
  // column) from the start cell instead, redrawn as the finger moves, so a
  // slipping finger cannot hit neighbours. The setting is per device; it is
  // on by default for touch screens.
  const OPTION_IDS = [["autocross", "autocross"], ["straight", "straight"], ["showProblems", "showproblems"]];
  const onOff = (v) => (v ? "an" : "aus");
  function applyPrefs() {
    for (const [k, id] of OPTION_IDS) $(id).checked = prefs[k];
    $("options-state").textContent =
      `Nachbarn kreuzen: ${onOff(prefs.autocross)} · Fat-Finger-Schutz: ${onOff(prefs.straight)} · Regelverstöße: ${onOff(prefs.showProblems)} · `;
  }
  applyPrefs();
  if (d.served) $("options-link").hidden = false;
  else $("play-options").hidden = false;
  for (const [k, id] of OPTION_IDS) {
    $(id).onchange = () => {
      prefs[k] = $(id).checked;
      L.setPref(k, prefs[k]);
      applyPrefs();
      if (mode === "play") setMode("play"); // board, problems and legend
    };
  }
  // Changed on /einstellungen in another tab: take over at once. The star
  // symbol too, by rebuilding the cells' symbol layer.
  window.addEventListener("storage", (ev) => {
    const k = L.prefOfKey(ev.key);
    if (k) {
      prefs[k] = L.pref(k);
      applyPrefs();
      if (mode === "play") setMode("play");
    } else if (ev.key === L.MARK_KEY) {
      showMark(L.markSpec(L.loadMark(), markOpts));
    }
  });
  // Draws the stars with spec from now on, by rebuilding the cells' symbol layer.
  function showMark(spec) {
    mark = spec;
    const tpl = L.markSVG(mark, document);
    L.markClasses(boardEl, mark);
    for (const e of cellEls) {
      e.classList.remove("fresh"); // its clip belongs to the old symbol
      e.querySelector(".sym").replaceChildren(tpl.cloneNode(true));
    }
  }
  // Under "random", starting anew brings another symbol.
  function rerollMark() {
    const stored = L.loadMark();
    if (L.isRandomMark(stored)) showMark(L.markSpec(stored, { ...markOpts, not: mark.id }));
  }
  let drag = null; // {start, last, erase, before, moved, straight, x, y, cell}
  const cellAt = (ev) => {
    const e = document.elementFromPoint ? document.elementFromPoint(ev.clientX, ev.clientY) : ev.target;
    const c = e && e.closest ? e.closest(".cell") : null;
    return c ? +c.dataset.i : -1;
  };
  boardEl.addEventListener("pointerdown", (ev) => {
    const c = ev.target.closest(".cell");
    if (!c || mode !== "play" || locked() || ev.button !== 0) return;
    ev.preventDefault();
    const i = +c.dataset.i;
    drag = {
      start: i, last: i, erase: marks[i] === L.CROSS, before: marks, moved: false, shift: ev.shiftKey,
      straight: prefs.straight, x: ev.clientX, y: ev.clientY, cell: c.getBoundingClientRect().width,
    };
  });
  document.addEventListener("pointermove", (ev) => {
    if (!drag) return;
    const i = drag.straight ? L.snapTarget(d, drag.start, ev.clientX - drag.x, ev.clientY - drag.y, drag.cell) : cellAt(ev);
    if (i < 0 || i === drag.last) return;
    if (!drag.moved) {
      drag.moved = true;
      overlay = null;
    }
    if (drag.straight) {
      marks = Uint8Array.from(drag.before);
      L.paint(marks, L.line(d, drag.start, i), drag.erase);
    } else {
      if (marks === drag.before) marks = Uint8Array.from(marks);
      L.paint(marks, L.line(d, drag.last, i), drag.erase);
    }
    drag.last = i;
    renderPlay();
  });
  const endDrag = (ev) => {
    if (!drag) return;
    const dr = drag;
    drag = null;
    if (!dr.moved) {
      if (ev.type === "pointerup") clickCell(dr.start, dr.shift || ev.shiftKey);
      return;
    }
    if (coop) { // one drag is one bundle of changes
      coopCommit(L.diffOps(dr.before, marks));
      return;
    }
    if (marks.some((v, i) => v !== dr.before[i])) {
      undoStack.push(dr.before);
      redoStack.length = 0;
    }
    renderPlay();
  };
  document.addEventListener("pointerup", endDrag);
  document.addEventListener("pointercancel", endDrag);
  boardEl.addEventListener("contextmenu", (ev) => {
    const c = ev.target.closest(".cell");
    if (!c || mode !== "play" || locked()) return;
    ev.preventDefault();
    clickCell(+c.dataset.i, true);
  });
  $("undo").onclick = () => {
    if (locked() || !undoStack.length) return;
    if (coop) return coopUndo(undoStack, redoStack);
    redoStack.push(marks);
    marks = undoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("redo").onclick = () => {
    if (locked() || !redoStack.length) return;
    if (coop) return coopUndo(redoStack, undoStack);
    undoStack.push(marks);
    marks = redoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("reset").onclick = () => {
    if (party) { // only the marks: time and hints count on
      if (locked() || !marks.some((v) => v !== L.FREE)) return;
      if (coop && !confirm("Das Brett für alle löschen?")) return;
      rerollMark();
      setMarks(new Uint8Array(n * n));
      return;
    }
    rerollMark();
    elapsed = 0;
    hintCount = 0;
    paid.clear();
    done = false;
    runningSince = null;
    clockRun(!document.hidden);
    if (marks.some((v) => v !== L.FREE)) setMarks(new Uint8Array(n * n));
    else renderPlay();
  };
  document.addEventListener("visibilitychange", () => { clockRun(!document.hidden); store(); });
  window.addEventListener("pagehide", store);
  setInterval(() => {
    if (runningSince !== null) { drawClock(); store(); }
    if (party && (!partyStarted() || party.live !== partyLive())) renderPlay(); // countdown, start
  }, 1000);
  clockRun(!document.hidden);
  $("hint").onclick = toggleHint;
  $("check").onclick = checkMistakes;

  // ---------- watch mode ----------
  let prefix = L.prefixStates(d, d.steps);
  let expanded = d.steps.map((s) => !!s.hypothesis);
  // A position is {k} (board after k steps, explaining step k-1) or {k, j}
  // (inside the refuted chain of step k-1, after its sub-steps 0..j). The chain
  // comes before its conclusion: "Angenommen … Widerspruch … also X".
  let positions = [];
  let cursor = 0;
  let timer = null;
  const rowEls = new Map(); // "k" or "k.j" → row element

  function buildPositions() {
    const cur = positions[cursor];
    positions = [{ k: 0 }];
    d.steps.forEach((s, i) => {
      if (s.hypothesis && expanded[i]) s.hypothesis.steps.forEach((_, j) => positions.push({ k: i + 1, j }));
      positions.push({ k: i + 1 });
    });
    if (cur) {
      const same = positions.findIndex((p) => p.k === cur.k && p.j === cur.j);
      cursor = same >= 0 ? same : positions.findIndex((p) => p.k === cur.k && p.j === undefined);
    }
  }

  function buildList() {
    const ol = $("steps");
    ol.replaceChildren();
    rowEls.clear();
    const start = el("li");
    const r0 = el("div", "row");
    r0.append(el("span", "num", "0"), el("span"), el("span", "text muted", "Leeres Brett"));
    r0.onclick = () => go({ k: 0 });
    start.append(r0);
    ol.append(start);
    rowEls.set("0", r0);
    d.steps.forEach((s, i) => {
      const li = el("li");
      const row = el("div", "row");
      const text = elr("span", "text", s.reason || s.source);
      if (s.hypothesis) {
        const t = el("button", "toggle-chain", expanded[i] ? "Gedankengang ausblenden" : "Gedankengang zeigen");
        t.onclick = (ev) => {
          ev.stopPropagation();
          expanded[i] = !expanded[i];
          buildPositions();
          buildList();
          render();
        };
        text.append(t);
      }
      row.append(el("span", "num", i + 1), chip(s), text);
      row.onclick = () => go({ k: i + 1 });
      li.append(row);
      rowEls.set(String(i + 1), row);
      if (s.hypothesis && expanded[i]) {
        const chain = el("ol", "chain");
        s.hypothesis.steps.forEach((h, j) => {
          const cli = el("li");
          const crow = el("div", "row");
          crow.append(el("span", "num", `${i + 1}.${j + 1}`), chip(h), elr("span", "text", h.reason || h.source));
          crow.onclick = () => go({ k: i + 1, j });
          cli.append(crow);
          chain.append(cli);
          rowEls.set(`${i + 1}.${j}`, crow);
        });
        chain.append(elr("li", "end", "⇒ Widerspruch: " + s.hypothesis.contradiction));
        li.append(chain);
      }
      ol.append(li);
    });
  }

  function go(pos) {
    const i = positions.findIndex((p) => p.k === pos.k && p.j === pos.j);
    if (i >= 0) {
      cursor = i;
      render();
    }
  }

  function viewAt(pos) {
    const toSet = (ms) => new Set(ms.map(([r, c]) => L.idx(d, r, c)));
    if (pos.j === undefined) {
      if (pos.k === 0) return { cells: prefix[0], step: null, marks: {} };
      const step = d.steps[pos.k - 1];
      const refs = L.refs(d, step.reason);
      return { cells: prefix[pos.k], step, marks: { new: toSet(step.marks), ref: refs.units, refcell: refs.cells } };
    }
    const parent = d.steps[pos.k - 1];
    const h = parent.hypothesis;
    const cells = Uint8Array.from(prefix[pos.k - 1]);
    for (let j = 0; j <= pos.j; j++) L.apply(d, cells, h.steps[j]);
    const step = h.steps[pos.j];
    const refs = L.refs(d, step.reason);
    const marks = { new: toSet(step.marks), ref: refs.units, refcell: refs.cells };
    const last = pos.j === h.steps.length - 1;
    if (last) marks.bad = new Set((h.cells || []).map(([r, c]) => L.idx(d, r, c)));
    return { cells, step, parent, last, marks };
  }

  // A trace that arrived later (loadTrace).
  function setTrace() {
    prefix = L.prefixStates(d, d.steps);
    expanded = d.steps.map((s) => !!s.hypothesis);
    positions = [];
    cursor = 0;
    buildPositions();
    buildList();
  }

  function render() {
    $("panel-watch").classList.toggle("pending", !!(lib && lib.pending));
    if (lib && lib.pending && done) loadTrace(); // e.g. opened with #weg
    const pos = positions[cursor];
    const v = viewAt(pos);
    drawBoard(v);
    const cur = $("current");
    cur.replaceChildren();
    const head = el("div", "head");
    if (pos.k === 0) {
      head.append(el("span", "", `Start · ${d.steps.length} Schritte`));
      cur.append(head, el("p", "reason muted", "Leeres Brett. Mit ▶ oder → geht es Schritt für Schritt durch die Lösung."));
    } else {
      const num = pos.j === undefined ? `Schritt ${pos.k} von ${d.steps.length}` : `Gedankengang ${pos.k}.${pos.j + 1}`;
      head.append(chip(v.step), el("span", "", num));
      cur.append(head, elr("p", "reason", v.step.reason || v.step.source));
      if (v.last) cur.append(elr("p", "contra", "⇒ Widerspruch: " + v.parent.hypothesis.contradiction));
    }
    if (pos.j !== undefined) {
      const first = v.parent.hypothesis.steps[0].marks.find((m) => m[2] === 1);
      banner(first ? `Gedankenexperiment: Was wäre, wenn auf (${first[0]},${first[1]}) ein Stern stünde?` : "Gedankenexperiment", "what-if");
    } else if (d.status === "pending") banner(null);
    else if (pos.k === d.steps.length && d.status === "solved") banner("Gelöst.", "good");
    else if (pos.k === d.steps.length && d.status !== "solved") banner(d.status === "stuck" ? "Hier kommt der Solver nicht weiter." : "Widerspruch: " + (d.error || ""), "what-if");
    else banner(null);

    for (const r of rowEls.values()) r.classList.remove("active");
    const key = pos.j === undefined ? String(pos.k) : `${pos.k}.${pos.j}`;
    const row = rowEls.get(key);
    if (row) {
      row.classList.add("active");
      // Scroll only the list (scrollIntoView would also move the page).
      const list = $("steps");
      if (row.offsetTop < list.scrollTop) list.scrollTop = row.offsetTop - 8;
      else if (row.offsetTop + row.offsetHeight > list.scrollTop + list.clientHeight)
        list.scrollTop = row.offsetTop + row.offsetHeight - list.clientHeight + 8;
    }
    $("first").disabled = $("prev").disabled = cursor === 0;
    $("last").disabled = $("next").disabled = cursor === positions.length - 1;
    if (cursor === positions.length - 1) stop();
  }

  const PLAY = '<svg viewBox="0 0 16 16"><path d="M4 2v12l10-6z"/></svg> Abspielen';
  const PAUSE = '<svg viewBox="0 0 16 16"><path d="M3 2h3.5v12H3zM9.5 2H13v12H9.5z"/></svg> Pause';
  function stop() {
    clearInterval(timer);
    timer = null;
    $("play").innerHTML = PLAY;
  }
  function step(delta) {
    cursor = Math.max(0, Math.min(positions.length - 1, cursor + delta));
    render();
  }
  $("first").onclick = () => { stop(); cursor = 0; render(); };
  $("last").onclick = () => { stop(); cursor = positions.length - 1; render(); };
  $("prev").onclick = () => { stop(); step(-1); };
  $("next").onclick = () => { stop(); step(1); };
  $("play").onclick = () => {
    if (timer) return stop();
    if (cursor === positions.length - 1) cursor = 0;
    $("play").innerHTML = PAUSE;
    step(1);
    timer = setInterval(() => step(1), +$("speed").value);
  };
  $("speed").onchange = () => {
    if (timer) {
      clearInterval(timer);
      timer = setInterval(() => step(1), +$("speed").value);
    }
  };

  // ---------- tabs and keys ----------
  let mode = "play";
  function setMode(m) {
    mode = m;
    stop();
    $("tab-play").setAttribute("aria-selected", m === "play");
    $("tab-watch").setAttribute("aria-selected", m === "watch");
    $("panel-play").hidden = m !== "play";
    $("panel-watch").hidden = m !== "watch";
    document.body.classList.toggle("watch", m === "watch");
    $("legend").textContent =
      m === "play"
        ? "★ Stern · ✕ Kreuz · • automatisch gekreuzt" +
          (prefs.showProblems ? " · rot: Regelverstoß" : lib ? "" : " · rot: falsch laut Prüfen") +
          (lib ? "" : " · orange: Hinweis")
        : "Blauer Rahmen: in diesem Schritt gesetzt · abgedunkelt: nicht in der Erklärung genannt · rot: Widerspruch";
    if (m === "play") renderPlay();
    else render();
  }
  $("tab-play").onclick = () => setMode("play");
  $("tab-watch").onclick = () => setMode("watch");

  document.addEventListener("keydown", (ev) => {
    const t = ev.target.closest ? ev.target : null;
    if (t && t.closest("input, select, textarea")) return;
    if (t && ev.key === " " && t.closest("button")) return; // the button handles it
    if (mode === "watch") {
      const k = { ArrowRight: () => $("next").click(), ArrowLeft: () => $("prev").click(), Home: () => $("first").click(), End: () => $("last").click(), " ": () => $("play").click() }[ev.key];
      if (k) {
        ev.preventDefault();
        k();
      }
    } else if (ev.ctrlKey || ev.metaKey) {
      if (ev.key === "z" && !ev.shiftKey) { ev.preventDefault(); $("undo").click(); }
      if (ev.key === "y" || (ev.key === "z" && ev.shiftKey) || ev.key === "Z") { ev.preventDefault(); $("redo").click(); }
    }
  });

  // ---------- party mode: a round played together (d.party) ----------
  // The server's state arrives as events (Party.live, party.js): the common
  // start, the clock, our result and everybody's progress. The board opens at
  // the start; solving or giving up is reported, and so is progress (stars set,
  // hints taken) while playing. A lost report is repeated with the next state.
  let partyView = null;
  function partyMe() {
    const s = party.snap;
    return s && s.you ? s.ranking.find((e) => e.id === s.you) : null;
  }
  function partyStarted() {
    return party.startLocal !== null && Date.now() >= party.startLocal;
  }
  function partyLive() {
    const me = partyMe();
    return !!me && party.snap.phase === "playing" && partyStarted() && (me.status === "playing" || me.status === "absent");
  }
  function partyPost(action, body) {
    return Party.post(d.party.code, action, { round: d.party.round, hints: hintCount, ...body });
  }
  function partyError(e) {
    if (partyView) partyView.error(e.message);
  }

  // The solution with every other cell crossed, shown after giving up.
  function solutionView() {
    const cells = new Uint8Array(n * n).fill(L.CROSS), auto = new Uint8Array(n * n).fill(1);
    if (d.solution) d.solution.forEach((c, r) => { cells[L.idx(d, r, c)] = L.STAR; auto[L.idx(d, r, c)] = 0; });
    return { cells, auto, marks: {} };
  }

  // [text, class] for the banner while not solved, or null.
  function partyBanner() {
    const s = party.snap;
    if (party.gaveUp) return [coop ? "Gemeinsam aufgegeben. Die Lösung ist eingeblendet." : "Aufgegeben. Die Lösung ist eingeblendet.", "what-if"];
    if (!s) return ["Verbinde mit der Party …", ""];
    const me = partyMe();
    if (!me) return ["Du spielst in dieser Runde nicht mit.", "what-if"];
    if (!partyStarted()) return [`Gleich geht es los: ${Math.max(1, Math.ceil((party.startLocal - Date.now()) / 1000))} …`, ""];
    if (s.phase !== "playing") return ["Die Runde ist vorbei.", "what-if"];
    if (coop && s.coop && s.coop.votes.length) {
      const k = s.coop.votes.length, m = s.ranking.filter((e) => e.status === "playing").length;
      return [`${k} von ${m} wollen aufgeben. Die Runde endet, wenn alle dafür sind.`, ""];
    }
    return null;
  }

  function partyTabs() {
    const open = done || party.gaveUp;
    $("tab-watch").hidden = !open;
    if (!open && mode === "watch") setMode("play");
  }

  function partySolve() {
    if (party.solving) return;
    party.solving = true;
    const eff = L.effective(d, marks, prefs.autocross);
    partyPost("solve", { stars: L.starColumns(d, eff.cells) })
      .then((r) => {
        elapsed = r.result.timeMs; // the server's time counts
        runningSince = null;
        hintCount = Math.max(hintCount, r.result.hints);
        renderPlay();
      })
      .catch(partyError)
      .finally(() => { party.solving = false; });
  }

  let partyTimer = null;
  function partyReport() {
    party.live = partyLive();
    if (coop || !party.live || partyTimer) return;
    partyTimer = setTimeout(() => {
      partyTimer = null;
      if (!partyLive()) return;
      let stars = 0;
      for (const v of marks) if (v === L.STAR) stars++;
      if (stars === party.sentStars && hintCount === party.sentHints) return;
      party.sentStars = stars;
      party.sentHints = hintCount;
      partyPost("progress", { count: stars }).catch(() => { party.sentStars = -1; });
    }, 700);
  }

  function partyGiveUp() {
    if (coop) return coopVote();
    if (locked() || !confirm("Wirklich aufgeben? Die Lösung wird angezeigt, und du landest hinter allen, die lösen.")) return;
    party.gaveUp = true;
    clockStop();
    overlay = null;
    message(...intro.map((x) => x.cloneNode(true)));
    partyPost("giveup", {}).catch(partyError);
    renderPlay();
  }

  function partySnap(s, at) {
    if (coop) { // event stream and replies may overtake each other
      if (s.version < coop.version) return;
      coop.version = s.version;
    }
    party.snap = s;
    if (s.round !== d.party.round) { // a new round: the lobby sends us on
      location.href = "/party/" + d.party.code;
      return;
    }
    party.startLocal = at + s.startInMs - s.elapsedMs;
    if (coop) return coopSnap(s);
    const me = partyMe();
    if (me) {
      hintCount = Math.max(hintCount, me.hints);
      if (me.status === "gaveup") party.gaveUp = true;
      if (me.status === "solved") {
        elapsed = me.timeMs;
        runningSince = null;
        if (!done && !party.gaveUp && d.solution) { // e.g. storage cleared
          marks = new Uint8Array(n * n);
          d.solution.forEach((c, r) => { marks[L.idx(d, r, c)] = L.STAR; });
          done = true;
        }
      } else if (s.phase === "playing" && done && !party.gaveUp) {
        partySolve(); // the report got lost
      } else if (s.phase === "playing" && party.gaveUp && me.status !== "gaveup") {
        partyPost("giveup", {}).catch(partyError);
      } else if (!done && !party.gaveUp && s.phase === "playing") {
        elapsed = 0;
        runningSince = party.startLocal;
      } else {
        clockStop();
      }
    }
    renderPlay();
  }

  // ---------- coop mode: one board for all (d.party.mode "coop") ----------
  // The server keeps the board and orders everybody's changes (coop.go). We
  // show its board plus our own changes not yet confirmed (pending), so input
  // never waits for the network. Changes are operations [num, i, from, to]
  // (L.applyOps): a change based on an outdated view of a cell is rejected
  // instead of overwriting what somebody else did. One request at a time
  // keeps our operations in order; repeating one after a lost reply is
  // harmless, the server skips numbers it has seen (acked).
  function coopShow() {
    marks = L.applyOps(coop.server, coop.pending).cells;
    renderPlay();
  }

  // ops: [i, from, to] as seen on the board now; one undo step.
  function coopCommit(ops, undoable = true) {
    overlay = null;
    if (ops.length && undoable) {
      undoStack.push(ops);
      redoStack.length = 0;
    }
    for (const op of ops) coop.pending.push([++coop.nextNum, ...op]);
    coopShow();
    coopFlush();
  }

  // Undo and redo take back only our own changes: a cell somebody else
  // changed since is left alone (and blinks).
  function coopUndo(from, to) {
    const ops = from.pop().map(([i, a, b]) => [i, b, a]);
    const ok = ops.filter(([i, a]) => marks[i] === a);
    flash(ops.filter(([i, a]) => marks[i] !== a).map(([i]) => i), "conflict");
    if (ok.length) to.push(ok.map(([i, a, b]) => [i, b, a]));
    coopCommit(ok, false);
  }

  function coopFlush() {
    if (coop.sending || !coop.pending.length || !partyLive()) return;
    coop.sending = true;
    partyPost("mark", { ops: coop.pending.slice() })
      .then((r) => {
        coop.sending = false;
        partySnap(r.snapshot, Date.now());
        coopFlush();
      })
      .catch((e) => {
        coop.sending = false;
        if (e.status === 0) { // no connection: try again
          setTimeout(coopFlush, 1500);
          return;
        }
        coop.pending = []; // e.g. the round is over: the server's board counts
        partyError(e);
        coopShow();
      });
  }

  // A new state from the server: its board replaces ours, our confirmed
  // operations drop out and the rest is replayed on top.
  function coopSnap(s) {
    const c = s.coop;
    if (c) {
      const first = !coop.ready;
      coop.server = L.decodeBoard(c.board);
      if (first) coop.nextNum = Math.max(coop.nextNum, c.acked);
      coop.ready = true;
      // Our own changes that did not make it (somebody was faster) blink red,
      // once each: as soon as the new board shows they cannot apply any more,
      // or when the server confirms that (only a cell's last change counts).
      const last = new Map();
      for (const op of coop.pending) if (op[0] <= c.acked) last.set(op[1], op);
      const lostOps = [...last.values()].filter((op) => coop.server[op[1]] !== op[3]);
      coop.pending = coop.pending.filter((op) => op[0] > c.acked);
      const doomed = new Set(L.applyOps(coop.server, coop.pending).rejected);
      lostOps.push(...coop.pending.filter((op) => doomed.has(op[0])));
      const lost = [];
      for (const op of lostOps) {
        if (!coop.lost.has(op[0])) lost.push(op[1]);
        coop.lost.add(op[0]);
      }
      coop.voted = !!s.you && c.votes.includes(s.you);
      if (c.solved) done = true;
      if (!drag) { // a drag in progress keeps its view until it ends
        const before = marks;
        marks = L.applyOps(coop.server, coop.pending).cells;
        if (!first) flash(L.diffOps(before, marks).map(([i]) => i).filter((i) => !lost.includes(i)), "remote");
      }
      flash(lost, "conflict");
    }
    const end = s.ranking.length ? s.ranking[0].timeMs : 0;
    if (c && c.solved) {
      elapsed = end;
      runningSince = null;
    } else if (s.phase === "playing") {
      elapsed = 0;
      runningSince = party.startLocal;
    } else { // given up together
      party.gaveUp = true;
      elapsed = end;
      runningSince = null;
    }
    renderPlay();
    coopFlush();
  }

  function coopVote() {
    if (locked()) return;
    if (!coop.voted && !confirm("Für Aufgeben stimmen? Die Runde endet, wenn alle dafür sind. Bis dahin geht es weiter.")) return;
    coop.voted = !coop.voted;
    renderPlay();
    partyPost("vote", { giveUp: coop.voted })
      .then((r) => partySnap(r.snapshot, Date.now()))
      .catch(partyError);
  }

  // Lets cells blink briefly: "remote" for changes by others, "conflict"
  // for our own changes that lost.
  function flash(cells, cls) {
    for (const i of cells) {
      const e = cellEls[i];
      e.classList.remove(cls);
      void e.offsetWidth; // restarts the animation
      e.classList.add(cls);
      clearTimeout(e[cls]);
      e[cls] = setTimeout(() => e.classList.remove(cls), 1200);
    }
  }

  if (party) {
    $("party-box").hidden = false;
    $("party-lobby").href = "/party/" + d.party.code;
    $("giveup").hidden = false;
    $("giveup").onclick = partyGiveUp;
    $("reset").textContent = "Alles löschen";
    $("reset").title = coop ? "Löscht das Brett für alle; die Zeit läuft weiter" : "Löscht deine Markierungen; Zeit und Hinweise zählen weiter";
    if (coop) { // no hints when playing together
      $("hint").hidden = $("check").hidden = true;
      $("giveup").title = "Stimme fürs Aufgeben; die Runde endet, wenn alle dafür sind";
    }
    elapsed = 0; // the server's clock; our result's time once it is known
    if (location.hash === "#weg") history.replaceState(null, "", location.pathname);
    partyView = Party.live(d.party.code, $("party-panel"), {
      onSnap: partySnap,
      onGone() { $("party-lobby").parentElement.hidden = true; }, // the panel shows the ways on
    });
  }

  // ---------- served by `starbattle serve`: new and pasted puzzles ----------
  if (d.served) {
    $("site-nav").hidden = false;
    $(party ? "nav-party" : lib ? "nav-library" : "nav-solo").setAttribute("aria-current", "page");
  }
  if (d.served && !party) {
    const sizes = $("newgame-n");
    for (let k = 4; k <= 14; k++) {
      const o = el("option", null, `${k}×${k}`);
      o.value = k;
      o.selected = k === n;
      sizes.append(o);
    }
    $("newgame").hidden = false;
    const dlg = $("paste");
    $("paste-open").onclick = () => {
      $("paste-text").value = d.text || "";
      dlg.showModal();
      $("paste-text").select();
    };
    $("paste-cancel").onclick = () => dlg.close();
    // Rows joined by "/" give a short, readable link.
    dlg.querySelector("form").addEventListener("submit", () => {
      const t = $("paste-text");
      t.value = t.value.split("\n").map((r) => r.replace(/\s+/g, "")).filter((r) => r && !r.startsWith("#")).join("/");
    });
  }

  buildPositions();
  buildList();
  setMode(location.hash === "#weg" && !(lib && !done) ? "watch" : "play");
}
