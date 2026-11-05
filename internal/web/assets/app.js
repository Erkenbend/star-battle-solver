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

  return { FREE, CROSS, STAR, idx, implied, effective, units, problems, mistakes, solved, apply, prefixStates, nextHint, refs, ruleLabel, line, paint, colorSegments };
})();

if (typeof module !== "undefined") module.exports = Logic;
if (typeof document !== "undefined") boot();

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
  let stride = Math.max(1, Math.round(n * 0.382));
  while (gcd(stride, n) !== 1) stride++;
  const regionHSL = (reg) => {
    const rank = (reg * stride) % n;
    if (n <= PALETTE.length) return PALETTE[Math.floor((rank * PALETTE.length) / n)];
    return [Math.round(rank * (360 / n) + 25) % 360, 60, rank % 2 ? 74 : 82];
  };
  const regionColor = (e, reg) => {
    const [h, s, l] = regionHSL(reg);
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
  const boardEl = $("board");
  boardEl.style.setProperty("--n", n);
  boardEl.append(el("div", "axis"));
  for (let c = 0; c < n; c++) boardEl.append(el("div", "axis top", c));
  const cellEls = [];
  for (let r = 0; r < n; r++) {
    boardEl.append(el("div", "axis", r));
    for (let c = 0; c < n; c++) {
      const reg = d.regions[r][c];
      const e = el("div", "cell");
      regionColor(e, reg);
      if (r === 0 || d.regions[r - 1][c] !== reg) e.classList.add("bt");
      if (c === 0 || d.regions[r][c - 1] !== reg) e.classList.add("bl");
      if (c === n - 1) e.classList.add("br");
      if (r === n - 1) e.classList.add("bb");
      e.append(el("span", "sym"));
      e.dataset.i = r * n + c;
      e.setAttribute("aria-label", `Feld (${r},${c}), Farbe ${d.letters[reg]}`);
      boardEl.append(e);
      cellEls.push(e);
    }
  }

  // view: {cells, auto?, marks: {new, hint, bad, refcell, ref} as Sets of indices}
  function drawBoard(view) {
    const m = view.marks || {};
    for (let i = 0; i < cellEls.length; i++) {
      const e = cellEls[i], v = view.cells[i];
      const auto = view.auto && view.auto[i];
      e.classList.toggle("star", v === L.STAR);
      e.classList.toggle("cross", v === L.CROSS && !auto);
      e.classList.toggle("auto", !!auto);
      for (const k of ["new", "hint", "bad", "refcell"]) e.classList.toggle(k, !!(m[k] && m[k].has(i)));
      // Cells outside the units named in the explanation are dimmed.
      e.classList.toggle("dim", !!(m.ref && m.ref.size && !m.ref.has(i)));
      e.lastChild.textContent = v === L.STAR ? "★" : v === L.CROSS ? (auto ? "•" : "✕") : "";
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
  const k = d.steps.length;
  const statusText =
    { solved: `gelöst in ${k} Schritten`, stuck: `steckt nach ${k} Schritten fest`, contradiction: `Widerspruch nach ${k} Schritten` }[d.status] || d.status;
  let summary = `${n}×${n} · Solver: ${statusText}`;
  if (d.stats.lookahead) summary += ` · ${d.stats.lookahead} Annahmen`;
  if (d.stats.guesses) summary += ` · ${d.stats.guesses}× geraten`;
  $("summary").textContent = summary;

  // ---------- storage (per-viewer convenience only) ----------
  const key = "starbattle:" + n + ":" + d.regions.map((row) => row.join(",")).join(";");
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
  if (saved && typeof saved.auto === "boolean") $("autocross").checked = saved.auto;
  const undoStack = [], redoStack = [];
  let overlay = null; // {hint?, bad?} shown until the next change
  let pendingHint = null;

  function setMarks(next) {
    undoStack.push(marks);
    redoStack.length = 0;
    marks = next;
    overlay = null;
    pendingHint = null;
    renderPlay();
  }

  function clickCell(i, star) {
    const next = Uint8Array.from(marks);
    if (star) next[i] = marks[i] === L.STAR ? L.FREE : L.STAR;
    else next[i] = (marks[i] + 1) % 3; // free → cross → star → free
    setMarks(next);
  }

  function renderPlay() {
    const eff = L.effective(d, marks, $("autocross").checked);
    const probs = L.problems(d, eff.cells);
    const bad = new Set(probs.flatMap((p) => p.cells));
    const view = { cells: eff.cells, auto: eff.auto, marks: { bad } };
    if (overlay) Object.assign(view.marks, overlay);
    drawBoard(view);

    const list = $("problems");
    list.replaceChildren(...probs.map((p) => elr("li", "", p.msg)));
    $("undo").disabled = undoStack.length === 0;
    $("redo").disabled = redoStack.length === 0;
    if (L.solved(d, eff.cells)) banner("Gelöst! Alle Sterne sitzen richtig.", "good");
    else banner(null);
    save({ marks: [...marks], auto: $("autocross").checked });
  }

  function message(...nodes) {
    $("play-msg").replaceChildren(...nodes);
  }

  function showHint() {
    const eff = L.effective(d, marks, $("autocross").checked);
    const wrong = L.mistakes(d, eff.cells);
    if (wrong.length) {
      const show = el("button", "", "Fehler zeigen");
      show.onclick = () => { overlay = { bad: new Set(wrong) }; renderPlay(); };
      const p = el("p", "", wrong.length === 1 ? "Ein Feld passt nicht zur Lösung. Erst das korrigieren, dann gibt es den nächsten Hinweis." : `${wrong.length} Felder passen nicht zur Lösung. Erst das korrigieren, dann gibt es den nächsten Hinweis.`);
      const a = el("div", "actions");
      a.append(show);
      message(p, a);
      return;
    }
    const h = L.nextHint(d, eff.cells);
    if (!h) {
      message(el("p", "", L.solved(d, eff.cells) ? "Schon gelöst." : "Hier kommt der Solver auch nicht weiter."));
      return;
    }
    const step = d.steps[h.step];
    const refs = L.refs(d, step.reason);
    pendingHint = h;
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

  function checkMistakes() {
    if (!d.solution) {
      message(el("p", "", "Für dieses Puzzle ist keine eindeutige Lösung bekannt."));
      return;
    }
    const eff = L.effective(d, marks, $("autocross").checked);
    const wrong = L.mistakes(d, eff.cells);
    overlay = { bad: new Set(wrong) };
    message(el("p", "", wrong.length ? `${wrong.length === 1 ? "Ein Feld ist" : wrong.length + " Felder sind"} falsch (rot markiert).` : "Bisher alles richtig."));
    renderPlay();
  }

  // Pointer input: a press without moving to another cell is a click (cycle
  // free → cross → star). Moving on paints crosses over every cell passed, or
  // removes them if the press started on a cross. One drag is one undo step.
  let drag = null; // {start, last, erase, before, moved}
  const cellAt = (ev) => {
    const e = document.elementFromPoint ? document.elementFromPoint(ev.clientX, ev.clientY) : ev.target;
    const c = e && e.closest ? e.closest(".cell") : null;
    return c ? +c.dataset.i : -1;
  };
  boardEl.addEventListener("pointerdown", (ev) => {
    const c = ev.target.closest(".cell");
    if (!c || mode !== "play" || ev.button !== 0) return;
    ev.preventDefault();
    const i = +c.dataset.i;
    drag = { start: i, last: i, erase: marks[i] === L.CROSS, before: marks, moved: false, shift: ev.shiftKey };
  });
  document.addEventListener("pointermove", (ev) => {
    if (!drag) return;
    const i = cellAt(ev);
    if (i < 0 || i === drag.last) return;
    if (!drag.moved) {
      drag.moved = true;
      marks = Uint8Array.from(marks);
      overlay = null;
      pendingHint = null;
    }
    L.paint(marks, L.line(d, drag.last, i), drag.erase);
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
    if (!c || mode !== "play") return;
    ev.preventDefault();
    clickCell(+c.dataset.i, true);
  });
  $("undo").onclick = () => {
    if (!undoStack.length) return;
    redoStack.push(marks);
    marks = undoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("redo").onclick = () => {
    if (!redoStack.length) return;
    undoStack.push(marks);
    marks = redoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("reset").onclick = () => {
    if (marks.some((v) => v !== L.FREE)) setMarks(new Uint8Array(n * n));
  };
  $("hint").onclick = showHint;
  $("check").onclick = checkMistakes;
  $("autocross").onchange = () => renderPlay();

  // ---------- watch mode ----------
  const prefix = L.prefixStates(d, d.steps);
  const expanded = d.steps.map((s) => !!s.hypothesis);
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

  function render() {
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
    } else if (pos.k === d.steps.length && d.status === "solved") banner("Gelöst.", "good");
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
        ? "★ Stern · ✕ Kreuz · • automatisch gekreuzt · rot: Regelverstoß · orange: Hinweis"
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

  buildPositions();
  buildList();
  setMode(location.hash === "#weg" ? "watch" : "play");
}
