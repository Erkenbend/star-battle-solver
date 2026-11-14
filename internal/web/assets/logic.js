"use strict";

// Logic: pure functions on the page data, shared by every page with script
// (puzzle page, library, settings, designer) and tested with node (see
// logic_test.js). The puzzle page's UI is app.js, which runs after this.
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
  // else (the same rule as the server, internal/party/coop.go).
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

  // Hypothesis mode ("Annahme"): the marks tried out lie as a layer over the
  // board they started from (base), as a Map of cell → mark where they differ.
  function overlay(base, cells) {
    const over = new Map();
    for (let i = 0; i < cells.length; i++) if (base[i] !== cells[i]) over.set(i, cells[i]);
    return over;
  }
  function compose(base, over) {
    const out = Uint8Array.from(base);
    for (const [i, v] of over) out[i] = v;
    return out;
  }
  // The hypothesis' star (root) after a change from prev to next: it stays
  // while it is a star; else the first cell that just became a star and is
  // not one in base; else none (-1).
  function hypRoot(base, prev, next, root) {
    if (root >= 0 && next[root] === STAR) return root;
    for (let i = 0; i < next.length; i++) if (next[i] === STAR && prev[i] !== STAR && base[i] !== STAR) return i;
    return -1;
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

  // ---------- tutorial (/anleitung, see tutorial.go) ----------
  // The first of the tutorial's steps not done on cells, steps.length once the
  // puzzle is solved (however it got there).
  function tutorialStep(d, steps, cells) {
    if (solved(d, cells)) return steps.length;
    const all = (list, v) => (list || []).every(([r, c]) => cells[idx(d, r, c)] === v);
    const k = steps.findIndex((s) => s.solve || !(all(s.stars, STAR) && all(s.crosses, CROSS)));
    return k < 0 ? steps.length : k;
  }

  // Whether this browser should see the tutorial first, remembered in storage
  // so it is shown once. Anybody with other traces of the site in storage has
  // been here before and is not sent there. Without working storage, never:
  // nothing would remember it was shown.
  const TUTORIAL_KEY = "starbattle.tutorial";
  function tutorialDue(storage) {
    try {
      if (storage.getItem(TUTORIAL_KEY)) return false;
      let before = false;
      for (let i = 0; i < storage.length; i++) if (/^starbattle[.:]/.test(storage.key(i))) before = true;
      storage.setItem(TUTORIAL_KEY, "1");
      return !before && storage.getItem(TUTORIAL_KEY) === "1";
    } catch { return false; }
  }

  // Where to go after the tutorial: a path on this site, else the start page.
  const returnPath = (p) => (typeof p === "string" && /^\/(?![/\\])[^\s\\]*$/.test(p) ? p : "/");

  // A hard library puzzle's difficulty (1 to 3, see hardsearch.Rating) as
  // stars, "★★☆" (without the empty ones "★★", hard to read when small), and
  // in words; empty for none.
  const ratingStars = (k, empty = true) => (k >= 1 && k <= 3 ? "★".repeat(k) + (empty ? "☆".repeat(3 - k) : "") : "");
  const ratingText = (k) => (k >= 1 && k <= 3 ? `Schwierigkeit ${k} von 3` : "");

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

  return { FREE, CROSS, STAR, idx, implied, effective, units, problems, mistakes, starColumns, solved, apply, prefixStates, nextHint, refs, ruleLabel, line, snapTarget, paint, colorSegments, applyOps, diffOps, overlay, compose, hypRoot, decodeBoard, regionHSL, nextUnsolved, ratingStars, ratingText, tutorialStep, tutorialDue, returnPath, pref, setPref, prefOfKey, MARK_KEY, DEFAULT_MARK, RANDOM_MARK, markSpec, isRandomMark, loadMark, saveMark, markSVG, markClasses, markRuns, pickClip, playMark };
})();

if (typeof module !== "undefined") module.exports = Logic;
