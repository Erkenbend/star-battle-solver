"use strict";

// The puzzle page (page.html): builds the board and all controls from the
// page data, for solo play, library puzzles, party rounds and the tutorial.
// Runs after logic.js (Logic), only on pages with a board.
//
// This is the core: board, play mode (marks, undo, hints, checks), clock,
// storage, options, tabs and keys. The other parts each have a file and get
// the page object P (below) from boot(): hypothesis.js (Annahme), watch.js
// (Lösungsweg), tutorial.js (Anleitung) and round.js (a party round) with
// coop.js and race.js for its modes. They are function declarations inlined
// into the same script (web.go, pageJS), so boot() finds them.

if (typeof document !== "undefined" && document.getElementById("board")) boot();

function boot() {
  const L = Logic;
  const d = JSON.parse(document.getElementById("data").textContent);
  // A new visitor of a solo page or library puzzle gets the tutorial first,
  // once, and comes back here afterwards. Not in a party round: it runs.
  if (d.served && !d.party && !d.tutorial) {
    let due = false;
    try { due = L.tutorialDue(localStorage); } catch { /* storage blocked */ }
    if (due) {
      location.replace("/anleitung?weiter=" + encodeURIComponent(location.pathname + location.search + location.hash));
      return;
    }
  }
  const tut = !!d.tutorial; // the tutorial (/anleitung), see tutorial.js
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
    // Units with their star lose some color (option fadeDone, maybe only colors).
    const faded = prefs.fadeDone ? L.doneCells(d, view.cells, prefs.fadeColorsOnly) : new Set();
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
      for (const k of ["new", "hint", "bad", "refcell", "hyp", "hyproot"]) e.classList.toggle(k, !!(m[k] && m[k].has(i)));
      e.classList.toggle("done", faded.has(i));
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

  // ---------- the page for the other parts ----------
  // hypothesis.js, watch.js, tutorial.js and round.js (with coop.js and
  // race.js) see the page through P: the data, helpers to draw, and the play
  // state of this function, read and written through accessors.
  const P = {
    L, d, n, $, el, elr, chip, banner, drawBoard, boardEl, cellEls,
    get saved() { return saved; },
    get marks() { return marks; },
    set marks(v) { marks = v; },
    get done() { return done; },
    set done(v) { done = v; },
    get hintCount() { return hintCount; },
    set hintCount(v) { hintCount = v; },
    get elapsed() { return elapsed; },
    set elapsed(v) { elapsed = v; },
    get runningSince() { return runningSince; },
    set runningSince(v) { runningSince = v; },
    get overlay() { return overlay; },
    set overlay(v) { overlay = v; },
    get prefs() { return prefs; },
    get drag() { return drag; },
    get mode() { return mode; },
    get undoStack() { return undoStack; },
    get redoStack() { return redoStack; },
    get lib() { return lib; },
    get hyp() { return H; },
    get party() { return !!party; },
    get coop() { return !!coop; },
    locked: () => locked(),
    intro: () => intro.map((x) => x.cloneNode(true)),
    message, renderPlay, setMarks, setMode, clockStop, fmtTime, loadTrace,
  };
  const H = Hypothesis(P);
  const W = Watch(P);
  const T = tut ? Tutorial(P) : null;

  // ---------- summary ----------
  function drawSummary() {
    const rating = d.library && d.library.rating ? ` · Schwierigkeit: ${L.ratingStars(d.library.rating)}` : "";
    if (d.library && d.library.pending) { // the solver's result comes later
      $("summary").textContent = `${n}×${n} · ohne Hinweise` + rating;
      return;
    }
    if (tut) {
      $("summary").textContent = "Ein kleines Puzzle zum Ausprobieren";
      return;
    }
    if (d.party && d.party.mode === "clock") {
      $("summary").textContent = `${n}×${n} · Gegen die Uhr · ohne Hinweise`;
      return;
    }
    const k = d.steps.length;
    const statusText =
      { solved: `gelöst in ${k} Schritten`, stuck: `steckt nach ${k} Schritten fest`, contradiction: `Widerspruch nach ${k} Schritten` }[d.status] || d.status;
    let summary = `${n}×${n} · Solver: ${statusText}`;
    if (d.stats.lookahead) summary += ` · ${d.stats.lookahead} Annahmen`;
    if (d.stats.guesses) summary += ` · ${d.stats.guesses}× geraten`;
    if (!d.solution) summary += " · keine eindeutige Lösung";
    $("summary").textContent = summary + rating;
  }
  drawSummary();

  // ---------- storage (per-viewer convenience only) ----------
  // A party round and a library puzzle keep their own progress, apart from
  // solo play of the puzzle. Library puzzles go by their stable ID.
  const key = d.library
    ? "starbattle:library:" + d.library.id
    : (d.party ? `starbattle:party:${d.party.code}:${d.party.round}:` : "starbattle:") + n + ":" + d.regions.map((row) => row.join(",")).join(";");
  const load = () => {
    if (tut) return null; // starts afresh every time and keeps nothing
    try { return JSON.parse(localStorage.getItem(key) || "null"); } catch { return null; }
  };
  const save = (v) => {
    if (tut) return;
    try { localStorage.setItem(key, JSON.stringify(v)); } catch { /* private mode etc. */ }
  };

  // ---------- play mode ----------
  const saved = load();
  let marks = new Uint8Array(n * n);
  if (saved && Array.isArray(saved.marks) && saved.marks.length === n * n) marks = Uint8Array.from(saved.marks);
  // Options of this device (see L.pref), set on /einstellungen. A page saved
  // as a file cannot reach that page and shows them itself.
  const prefs = { autocross: L.pref("autocross"), straight: L.pref("straight"), showProblems: L.pref("showProblems"), fadeDone: L.pref("fadeDone"), fadeColorsOnly: L.pref("fadeColorsOnly") };
  if (tut) { // the steps cross by hand, rely on mistakes showing at once and name colors
    prefs.autocross = false;
    prefs.showProblems = true;
    prefs.fadeDone = false;
  }

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
  // A round played together (d.party), see round.js; its modes all on one
  // board (coop.js) and against the clock (race.js).
  const party = d.party ? PartyRound(P) : null;
  const coop = party && party.coop;
  const race = party && party.race;
  // A library puzzle (d.library) is a motif or a hard puzzle. A motif is
  // played like a solo puzzle, with hints and checks. A hard one is solved
  // without help (bare): no hints, no checks, and the solver's way only once
  // it is solved. Solved IDs of both are kept on this device for the overview
  // (/bibliothek), with the best result.
  const lib = d.library || null;
  const motif = !!lib && lib.kind === "motive";
  const bare = !!lib && !motif;
  const libSolvedKey = "starbattle:library:solved";
  function libSolved() {
    try {
      const v = JSON.parse(localStorage.getItem(libSolvedKey) || "{}");
      return v && typeof v === "object" ? v : {};
    } catch { return {}; }
  }
  // Keeps the result if it is better: fewer hints (a motif's), then less time.
  function libRecord(ms, hints) {
    const all = libSolved();
    const prev = all[lib.id];
    if (prev && Number.isFinite(prev.time)) {
      const ph = num(prev.hints);
      if (ph < hints || (ph === hints && prev.time <= ms)) return;
    }
    all[lib.id] = { time: Math.round(ms), at: new Date().toISOString().slice(0, 10) };
    if (hints) all[lib.id].hints = hints;
    try { localStorage.setItem(libSolvedKey, JSON.stringify(all)); } catch { /* private mode etc. */ }
  }
  if (bare) {
    $("hint").hidden = true;
    $("check").hidden = true;
    $("play-msg").append(el("p", "muted", "Bibliothek: ein besonders schweres Puzzle, an dem die einfachen Regeln scheitern. Hier gibt es keine Hinweise und kein Prüfen. Den Lösungsweg des Solvers siehst du erst, wenn du das Puzzle gelöst hast."));
  }
  // No input: solved, or in a party round given up, not started or not ours.
  const locked = () => done || (!!party && (party.gaveUp || !party.live()));
  function fmtTime(ms) {
    const s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    const ss = String(s % 60).padStart(2, "0");
    return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
  }
  const hintsText = (k) => (k === 1 ? "1 Hinweis" : `${k} Hinweise`);
  function drawClock() {
    if (race) return race.clock();
    $("clock").textContent = coop ? `⏱ ${fmtTime(clockMs())} · gemeinsam` : bare ? `⏱ ${fmtTime(clockMs())}` : `⏱ ${fmtTime(clockMs())} · ${hintsText(hintCount)}`;
  }
  const undoStack = [], redoStack = [];
  let overlay = null; // {hint?, bad?} shown until the next change

  // A hypothesis ("Annahme", see hypothesis.js) saved with the progress.
  if (!coop) H.restore(saved);
  const intro = [...$("play-msg").childNodes].map((x) => x.cloneNode(true));

  // merge: part of the last change, together one undo step.
  function setMarks(next, merge = false) {
    if (locked()) return; // solved puzzles are frozen
    if (H.active) return H.change(next, merge);
    if (coop) {
      coop.commit(L.diffOps(marks, next), true, merge);
      return;
    }
    if (!merge || !undoStack.length) undoStack.push(marks);
    redoStack.length = 0;
    marks = next;
    overlay = null;
    renderPlay();
  }

  // Tap on a cell (a press without dragging). A star directly (right click,
  // shift+click) toggles at once. A plain click on a symbol frees the cell at
  // once; on a free cell it waits briefly whether a second click follows: a
  // double click sets a star, a single one a cross. Meanwhile the cross is
  // only shown (see renderPlay); it counts, for the rules too, once settled,
  // so a double click passes no state that could flash red. A double click
  // on a cross makes it a star as one undo step; the second click on a star
  // just removed does nothing. tap: {i, was, timer} while a second click may
  // follow.
  const DOUBLE_CLICK_MS = 250;
  let tap = null;
  function clickCell(i, star) {
    if (star) {
      flushTap();
      const next = Uint8Array.from(marks);
      next[i] = marks[i] === L.STAR ? L.FREE : L.STAR;
      return setMarks(next);
    }
    if (tap && tap.i === i) { // the second click of a double click
      const was = tap.was;
      endTap();
      if (was === L.STAR) return;
      const next = Uint8Array.from(marks);
      next[i] = L.STAR;
      return setMarks(next, was === L.CROSS);
    }
    flushTap();
    const was = marks[i];
    if (was !== L.FREE) {
      const next = Uint8Array.from(marks);
      next[i] = L.FREE;
      setMarks(next);
    }
    tap = { i, was, timer: setTimeout(flushTap, DOUBLE_CLICK_MS) };
    boardEl.toggleAttribute("data-pending", was === L.FREE); // the UI check waits for it
    if (was === L.FREE) renderPlay(); // the placeholder cross
  }
  function endTap() {
    const shown = tapCross();
    clearTimeout(tap.timer);
    tap = null;
    boardEl.removeAttribute("data-pending");
    if (shown && mode === "play") renderPlay(); // in case nothing replaces it
  }
  // A click waiting for its double: its cell is shown as a cross.
  const tapCross = () => !!tap && tap.was === L.FREE && marks[tap.i] === L.FREE && !locked();
  // No second click (in time): a click on a free cell was a single one.
  function flushTap() {
    if (!tap) return;
    const { i, was } = tap;
    endTap();
    if (was !== L.FREE || marks[i] !== L.FREE) return; // a partner may have been quicker
    const next = Uint8Array.from(marks);
    next[i] = L.CROSS;
    setMarks(next);
  }

  function renderPlay() {
    const eff = L.effective(d, marks, prefs.autocross);
    if (H.active && !locked() && L.solved(d, eff.cells)) return H.accept(); // a normal solve
    // Rule violations (two stars touching, a full row …) unless switched off;
    // "Prüfen" still marks wrong cells, as asked for explicitly.
    const probs = prefs.showProblems ? L.problems(d, eff.cells) : [];
    const bad = new Set(probs.flatMap((p) => p.cells));
    let shown = eff; // with the placeholder cross of a click waiting for its double
    if (tapCross()) {
      const m = Uint8Array.from(marks);
      m[tap.i] = L.CROSS;
      shown = L.effective(d, m, prefs.autocross);
    }
    const view = { cells: shown.cells, auto: shown.auto, marks: { bad } };
    if (overlay) Object.assign(view.marks, overlay);
    H.decorate(view, shown.cells); // the marks tried out
    drawBoard(party && party.gaveUp ? party.solutionView() : view);

    const list = $("problems");
    list.replaceChildren(...probs.map((p) => elr("li", "", p.msg)));
    // Once solved, the puzzle is frozen until "Neu beginnen": the board
    // ignores input and scrolls on touch like the rest of the page.
    const frozen = locked() || L.solved(d, eff.cells);
    document.body.classList.toggle("frozen", frozen);
    if (party) party.controls(frozen);
    $("undo").disabled = frozen || (H.active ? H.undo : undoStack).length === 0;
    $("redo").disabled = frozen || (H.active ? H.redo : redoStack).length === 0;
    $("hint").disabled = frozen || H.active; // they would judge the hypothesis by the solution
    $("check").disabled = frozen || H.active;
    H.controls(frozen);
    $("hint").textContent = overlay && overlay.hint ? "Hinweis ausblenden" : "Hinweis";
    // In a coop round the server says when the board is solved.
    const solved = L.solved(d, eff.cells) && !(party && party.gaveUp) && (!coop || done);
    if (solved && !done) {
      clockStop();
      done = true;
      if (party) party.solve();
    }
    if (solved && !race && !tut) { // against the clock, the party banner tells how it goes on; in the tutorial its card
      if (lib) libRecord(clockMs(), bare ? 0 : hintCount);
      const with_ = bare ? "ohne Hilfe" : !hintCount ? "ohne Hinweise" : hintCount === 1 ? "mit 1 Hinweis" : `mit ${hintCount} Hinweisen`;
      banner(coop ? `Gemeinsam gelöst in ${fmtTime(clockMs())}! Alle Sterne sitzen richtig.` : `Gelöst in ${fmtTime(clockMs())} ${with_}! Alle Sterne sitzen richtig.`, "good");
      if (!party) {
        const share = el("button", "share", "Ergebnis kopieren");
        share.title = bare ? "Text mit Zeit und Link zu diesem Puzzle in die Zwischenablage" : "Text mit Zeit, Hinweisen und Link in die Zwischenablage";
        share.onclick = () => copyText(shareText(with_), share);
        $("banner").append(" ", share);
      }
      if (lib) {
        const next = L.nextUnsolved(lib.entries, libSolved(), lib.id);
        const a = el("a", "next", next ? `Weiter: ${next.name}` : "Alle gelöst: zur Bibliothek");
        a.href = next ? "/bibliothek?id=" + encodeURIComponent(next.id) : "/bibliothek#" + lib.kind;
        $("banner").append(" ", a);
      }
    } else {
      const b = party ? party.banner() : null;
      if (b) banner(b[0], b[1]);
      else if (H.active) banner(H.text(), "hypothesis");
      else banner(null);
    }
    drawClock();
    store();
    if (party) {
      party.tabs();
      party.report();
    }
    if (bare) {
      $("tab-watch").hidden = !done;
      if (done && lib.pending) loadTrace();
    }
    if (T) T.draw(eff.cells);
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
        W.setTrace();
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
        if (mode === "watch") W.render();
      });
  }

  // The text for sharing a result; with the page's link when it has a stable
  // one (served pages: /?n=8&seed=42 or /play?p=...), not for a local file.
  function shareText(with_) {
    const web = d.served && /^https?:$/.test(location.protocol);
    if (motif) {
      let t = `Star-Battle-Motiv „${lib.name}“ (${n}×${n}) in ${fmtTime(clockMs())} gelöst (${with_})!`;
      if (web) t += " " + location.origin + "/bibliothek?id=" + encodeURIComponent(lib.id);
      return t;
    }
    if (lib) {
      const stars = lib.rating ? `, ${L.ratingStars(lib.rating)}` : "";
      let t = `Schweres Star Battle ${lib.name} (${n}×${n}${stars}) aus der Bibliothek in ${fmtTime(clockMs())} gelöst, ${with_}!`;
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
    const v = { marks: [...H.committed()], time: Math.round(clockMs()), hints: hintCount, done, paid: [...paid] };
    H.save(v);
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

  // Pointer input: a press without moving to another cell is a click (see
  // clickCell: cross, star by double click, free on a symbol). Moving on paints crosses over every cell passed, or
  // removes them if the press started on a cross. One drag is one undo step.
  // With the fat-finger guard on, a drag draws one straight line (row or
  // column) from the start cell instead, redrawn as the finger moves, so a
  // slipping finger cannot hit neighbours. The setting is per device; it is
  // on by default for touch screens.
  const OPTION_IDS = [["autocross", "autocross"], ["straight", "straight"], ["showProblems", "showproblems"], ["fadeDone", "fadedone"], ["fadeColorsOnly", "fadecolorsonly"]];
  const onOff = (v) => (v ? "an" : "aus");
  function applyPrefs() {
    for (const [k, id] of OPTION_IDS) $(id).checked = prefs[k];
    $("fadecolorsonly").disabled = !prefs.fadeDone; // a part of fadeDone
    const fade = !prefs.fadeDone ? "aus" : prefs.fadeColorsOnly ? "nur Farben" : "an";
    $("options-state").textContent =
      `Nachbarn kreuzen: ${onOff(prefs.autocross)} · Fat-Finger-Schutz: ${onOff(prefs.straight)} · Regelverstöße: ${onOff(prefs.showProblems)} · Erledigtes blasser: ${fade} · `;
  }
  applyPrefs();
  if (!d.served) $("play-options").hidden = false;
  else if (!tut) $("options-link").hidden = false;
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
    if (k && tut) return; // fixed for the tutorial's steps
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
  // Any other input first settles a click still waiting for its double.
  document.addEventListener("pointerdown", (ev) => {
    const c = ev.target.closest && ev.target.closest(".cell");
    if (tap && !(c && +c.dataset.i === tap.i && ev.button === 0 && !ev.shiftKey)) flushTap();
  }, true);
  document.addEventListener("keydown", () => flushTap(), true);
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
      if (tap) { // a drag from the cell just clicked: that click was a single one
        flushTap();
        drag.before = marks;
      }
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
    if (H.active) {
      const next = marks;
      marks = dr.before;
      if (next.some((v, i) => v !== dr.before[i])) H.change(next);
      else renderPlay();
      return;
    }
    if (coop) { // one drag is one bundle of changes
      coop.commit(L.diffOps(dr.before, marks));
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
    if (locked()) return;
    if (H.active) return H.undoStep();
    if (!undoStack.length) return;
    if (coop) return coop.undo(undoStack, redoStack);
    redoStack.push(marks);
    marks = undoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("redo").onclick = () => {
    if (locked()) return;
    if (H.active) return H.redoStep();
    if (!redoStack.length) return;
    if (coop) return coop.undo(redoStack, undoStack);
    undoStack.push(marks);
    marks = redoStack.pop();
    overlay = null;
    renderPlay();
  };
  $("reset").onclick = () => {
    if (party) { // only the marks: time and hints count on
      if (locked() || !marks.some((v) => v !== L.FREE)) return;
      if (coop && H.committed().some((v) => v !== L.FREE) && !confirm("Das Brett für alle löschen?")) return;
      H.drop();
      rerollMark();
      if (marks.some((v) => v !== L.FREE)) setMarks(new Uint8Array(n * n));
      else renderPlay();
      return;
    }
    H.drop();
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
    if (race) drawClock();
    if (party) party.tick(); // countdown, start
  }, 1000);
  clockRun(!document.hidden);
  $("hint").onclick = toggleHint;
  $("check").onclick = checkMistakes;

  // ---------- tabs and keys ----------
  let mode = "play";
  function setMode(m) {
    mode = m;
    W.stop();
    $("tab-play").setAttribute("aria-selected", m === "play");
    $("tab-watch").setAttribute("aria-selected", m === "watch");
    $("panel-play").hidden = m !== "play";
    $("panel-watch").hidden = m !== "watch";
    document.body.classList.toggle("watch", m === "watch");
    $("legend").textContent =
      m === "play"
        ? "★ Stern · ✕ Kreuz · • automatisch gekreuzt" +
          (prefs.showProblems ? " · rot: Regelverstoß" : bare || race ? "" : " · rot: falsch laut Prüfen") +
          (bare || race ? "" : " · orange: Hinweis") +
          " · lila: Annahme"
        : "Blauer Rahmen: in diesem Schritt gesetzt · abgedunkelt: nicht in der Erklärung genannt · rot: Widerspruch";
    if (m === "play") renderPlay();
    else W.render();
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

  if (party) party.start();

  // ---------- served by `starbattle serve`: new puzzles ----------
  if (d.served) {
    $("site-nav").hidden = false;
    $(party ? "nav-party" : lib ? "nav-library" : tut ? "nav-help" : "nav-solo").setAttribute("aria-current", "page");
  }
  if (d.served && !party && !tut) {
    const sizes = $("newgame-n");
    for (let k = d.minN; k <= d.maxN; k++) {
      const o = el("option", null, `${k}×${k}`);
      o.value = k;
      o.selected = k === n;
      sizes.append(o);
    }
    $("newgame").hidden = false;
  }

  W.init();
  setMode(location.hash === "#weg" && !(bare && !done) ? "watch" : "play");
}
